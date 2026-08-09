package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// templates maps each page name to its own *template.Template instance.
// Each instance is parsed from layout.html + that page only, so
// {{define "content"}} blocks don't conflict across pages.
var templates map[string]*template.Template

func init() {
	layoutBytes, err := templatesFS.ReadFile("templates/layout.html")
	if err != nil {
		panic(fmt.Sprintf("failed to read layout template: %v", err))
	}

	pages := []string{"index.html", "queue.html", "create_queue.html", "metrics.html"}
	templates = make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		pageBytes, err := templatesFS.ReadFile("templates/" + page)
		if err != nil {
			panic(fmt.Sprintf("failed to read template %s: %v", page, err))
		}
		t := template.Must(template.New("layout").Parse(string(layoutBytes)))
		t = template.Must(t.New(page).Parse(string(pageBytes)))
		templates[page] = t
	}
}

// handler holds dependencies for UI HTTP handlers.
type handler struct {
	manager        *queue.QueueManager
	log            logger.LoggerInterface
	metricsEnabled bool
}

func newHandler(manager *queue.QueueManager, log logger.LoggerInterface, metricsEnabled bool) *handler {
	return &handler{manager: manager, log: log, metricsEnabled: metricsEnabled}
}

// pageData is the base data passed to every template.
type pageData struct {
	Title          string
	Queues         []queueListItem
	Queue          *queueDetailData
	Error          string
	Success        string
	MetricsEnabled bool
	Metrics        *metricsData
}

// --- Data models for templates ---

type queueListItem struct {
	Name      string
	IsFifo    bool
	Available int
	InFlight  int
	Delayed   int
	URL       string
}

type queueDetailData struct {
	Title          string
	Name           string
	IsFifo         bool
	URL            string
	ARN            string
	Attrs          []attrPair
	Tags           map[string]string
	Available      int
	InFlight       int
	Delayed        int
	Messages       []messageDisplay
	Error          string
	Success        string
	MetricsEnabled bool
}

type attrPair struct {
	Name  string
	Value string
}

type messageDisplay struct {
	MessageID     string
	ReceiptHandle string
	Body          string
	ReceiveCount  int
	SentTimestamp string
}

// --- Metrics data models for templates ---

type metricsData struct {
	APIRequests      []metricAPIRequest
	QueueSizes       []metricQueueSize
	MessagesSent     []metricCounter
	MessagesReceived []metricCounter
	MessagesDeleted  []metricCounter
	MoveTaskMessages []metricMoveTask
	MoveTaskActive   int
	Raw              string
}

type metricAPIRequest struct {
	Action   string
	Protocol string
	Count    int
}

type metricQueueSize struct {
	Queue string
	Type  string
	Size  int
}

type metricCounter struct {
	Queue string
	Count int
}

type metricMoveTask struct {
	SourceARN      string
	DestinationARN string
	Count          int
}

// --- HTML page handlers ---

// handleIndex renders the queue list page.
func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	queues := h.manager.ListQueues("")
	items := make([]queueListItem, 0, len(queues))
	for _, q := range queues {
		items = append(items, queueListItem{
			Name:      q.Name(),
			IsFifo:    q.IsFifo(),
			Available: q.ApproximateNumberOfMessages(),
			InFlight:  q.ApproximateNumberOfMessagesNotVisible(),
			Delayed:   q.ApproximateNumberOfMessagesDelayed(),
			URL:       q.URL(h.manager.NodeAddress(), h.manager.AccountID()),
		})
	}

	data := pageData{
		Title:  "Queues",
		Queues: items,
	}
	if e := r.URL.Query().Get("error"); e != "" {
		data.Error = e
	}
	if s := r.URL.Query().Get("success"); s != "" {
		data.Success = s
	}
	h.renderTemplate(w, "index.html", data)
}

// handleCreateQueueForm renders the create queue form.
func (h *handler) handleCreateQueueForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.renderTemplate(w, "create_queue.html", pageData{Title: "Create Queue"})
}

// handleCreateQueue handles POST /queues/create.
func (h *handler) handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/queues/new?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	queueName := r.FormValue("queueName")
	if queueName == "" {
		http.Redirect(w, r, "/queues/new?error=Queue+name+is+required", http.StatusSeeOther)
		return
	}

	attrs := queue.NewDefaultQueueAttributes()

	if r.FormValue("fifoQueue") == "on" {
		attrs.FifoQueue = true
		if !strings.HasSuffix(queueName, ".fifo") {
			http.Redirect(w, r, "/queues/new?error=FIFO+queue+names+must+end+with+.fifo", http.StatusSeeOther)
			return
		}
	}

	if r.FormValue("contentBasedDeduplication") == "on" {
		attrs.ContentBasedDeduplication = true
	}

	if v := r.FormValue("visibilityTimeout"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			attrs.VisibilityTimeout = n
		}
	}
	if v := r.FormValue("delaySeconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			attrs.DelaySeconds = n
		}
	}
	if v := r.FormValue("receiveMessageWaitTimeSeconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			attrs.ReceiveMessageWaitTimeSeconds = n
		}
	}

	// Optional RedrivePolicy
	dlqArn := r.FormValue("dlqArn")
	maxReceiveCountStr := r.FormValue("maxReceiveCount")
	if dlqArn != "" {
		maxReceiveCount, err := strconv.Atoi(maxReceiveCountStr)
		if err != nil || maxReceiveCount < 1 {
			http.Redirect(w, r, "/queues/new?error=Invalid+maxReceiveCount", http.StatusSeeOther)
			return
		}
		rpJSON, err := json.Marshal(map[string]any{
			"deadLetterTargetArn": dlqArn,
			"maxReceiveCount":     maxReceiveCount,
		})
		if err != nil {
			http.Redirect(w, r, "/queues/new?error=Failed+to+create+queue", http.StatusSeeOther)
			return
		}
		attrs.RedrivePolicy = string(rpJSON)
	}

	if _, err := h.manager.CreateQueue(queueName, attrs); err != nil {
		h.log.Errorf("failed to create queue %q: %v", queueName, err)
		http.Redirect(w, r, "/queues/new?error=Failed+to+create+queue", http.StatusSeeOther)
		return
	}

	h.log.Infof("created queue: %s", queueName)
	http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?success=Queue+created", http.StatusSeeOther)
}

// handleQueueRoutes dispatches queue-specific routes.
// Paths: /queues/{name}, /queues/{name}/delete, /queues/{name}/purge,
// /queues/{name}/messages, /queues/{name}/messages/{receiptHandle}/delete
func (h *handler) handleQueueRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/queues/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	queueName := parts[0]

	// POST /queues/{name}/delete
	if len(parts) == 2 && parts[1] == "delete" && r.Method == http.MethodPost {
		h.handleDeleteQueue(w, r, queueName)
		return
	}

	// POST /queues/{name}/purge
	if len(parts) == 2 && parts[1] == "purge" && r.Method == http.MethodPost {
		h.handlePurgeQueue(w, r, queueName)
		return
	}

	// POST /queues/{name}/messages
	if len(parts) == 2 && parts[1] == "messages" && r.Method == http.MethodPost {
		h.handleSendMessage(w, r, queueName)
		return
	}

	// POST /queues/{name}/messages/{receiptHandle}/delete
	if len(parts) == 4 && parts[1] == "messages" && parts[3] == "delete" && r.Method == http.MethodPost {
		h.handleDeleteMessage(w, r, queueName, parts[2])
		return
	}

	// GET /queues/{name}
	if len(parts) == 1 && r.Method == http.MethodGet {
		h.handleQueueDetail(w, r, queueName)
		return
	}

	http.NotFound(w, r)
}

// handleQueueDetail renders the queue detail page.
func (h *handler) handleQueueDetail(w http.ResponseWriter, r *http.Request, queueName string) {
	q, err := h.manager.LookupQueue(queueName)
	if err != nil {
		h.renderTemplate(w, "queue.html", queueDetailData{
			Title: "Queue",
			Name:  queueName,
			Error: fmt.Sprintf("Queue not found: %s", queueName),
		})
		return
	}

	attrs := q.Attributes()
	arn := q.ARN(h.manager.Region(), h.manager.AccountID())

	attrPairs := buildAttrPairs(attrs, q)

	// Receive messages for display (non-destructive peek via short visibility timeout)
	msgs, _ := q.Store().ReceiveMessages(r.Context(), 10, 1, 0)
	msgDisplays := make([]messageDisplay, 0, len(msgs))
	for _, m := range msgs {
		msgDisplays = append(msgDisplays, messageDisplay{
			MessageID:     m.MessageID,
			ReceiptHandle: m.ReceiptHandle,
			Body:          m.Body,
			ReceiveCount:  m.ApproximateReceiveCount,
			SentTimestamp: m.SentTimestamp.Format("2006-01-02 15:04:05"),
		})
	}

	data := queueDetailData{
		Title:     "Queue: " + q.Name(),
		Name:      q.Name(),
		IsFifo:    q.IsFifo(),
		URL:       q.URL(h.manager.NodeAddress(), h.manager.AccountID()),
		ARN:       arn,
		Attrs:     attrPairs,
		Tags:      q.Tags(),
		Available: q.ApproximateNumberOfMessages(),
		InFlight:  q.ApproximateNumberOfMessagesNotVisible(),
		Delayed:   q.ApproximateNumberOfMessagesDelayed(),
		Messages:  msgDisplays,
	}

	// Check for flash messages from redirect
	flashQuery := r.URL.Query()
	if e := flashQuery.Get("error"); e != "" {
		data.Error = e
	}
	if s := flashQuery.Get("success"); s != "" {
		data.Success = s
	}

	h.renderTemplate(w, "queue.html", data)
}

// handleDeleteQueue deletes a queue and redirects to the list.
func (h *handler) handleDeleteQueue(w http.ResponseWriter, r *http.Request, queueName string) {
	if err := h.manager.DeleteQueue(queueName); err != nil {
		h.log.Errorf("failed to delete queue %q: %v", queueName, err)
		http.Redirect(w, r, "/?error="+url.QueryEscape(fmt.Sprintf("Failed to delete queue %s", queueName)), http.StatusSeeOther)
		return
	}
	h.log.Infof("deleted queue: %s", queueName)
	http.Redirect(w, r, "/?success=Queue+deleted", http.StatusSeeOther)
}

// handlePurgeQueue purges all messages from a queue.
func (h *handler) handlePurgeQueue(w http.ResponseWriter, r *http.Request, queueName string) {
	q, err := h.manager.LookupQueue(queueName)
	if err != nil {
		http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?error=Queue+not+found", http.StatusSeeOther)
		return
	}
	if err := q.Store().Purge(r.Context()); err != nil {
		h.log.Errorf("failed to purge queue %q: %v", queueName, err)
		http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?error=Failed+to+purge", http.StatusSeeOther)
		return
	}
	h.log.Infof("purged queue: %s", queueName)
	http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?success=Queue+purged", http.StatusSeeOther)
}

// handleSendMessage sends a message to a queue from form data.
func (h *handler) handleSendMessage(w http.ResponseWriter, r *http.Request, queueName string) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/queues/"+queueName+"?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	q, err := h.manager.LookupQueue(queueName)
	if err != nil {
		http.Redirect(w, r, "/?error=Queue+not+found", http.StatusSeeOther)
		return
	}

	body := r.FormValue("body")
	if body == "" {
		http.Redirect(w, r, "/queues/"+queueName+"?error=Message+body+is+required", http.StatusSeeOther)
		return
	}

	msg := &types.Message{
		Body: body,
	}

	// FIFO-specific fields
	if q.IsFifo() {
		msg.MessageGroupID = r.FormValue("messageGroupId")
		msg.MessageDeduplicationID = r.FormValue("messageDeduplicationId")
	}

	delaySeconds := 0
	if d := r.FormValue("delaySeconds"); d != "" {
		if v, err := strconv.Atoi(d); err == nil {
			delaySeconds = v
		}
	}

	if err := q.Store().SendMessage(r.Context(), msg, delaySeconds); err != nil {
		h.log.Errorf("failed to send message to queue %q: %v", queueName, err)
		http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?error=Failed+to+send+message", http.StatusSeeOther)
		return
	}

	h.log.Infof("sent message to queue: %s", queueName)
	http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?success=Message+sent", http.StatusSeeOther)
}

// handleDeleteMessage deletes a specific message by receipt handle.
func (h *handler) handleDeleteMessage(w http.ResponseWriter, r *http.Request, queueName, receiptHandle string) {
	q, err := h.manager.LookupQueue(queueName)
	if err != nil {
		http.Redirect(w, r, "/?error=Queue+not+found", http.StatusSeeOther)
		return
	}

	if err := q.Store().DeleteMessage(r.Context(), receiptHandle); err != nil {
		h.log.Errorf("failed to delete message from queue %q: %v", queueName, err)
		http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?error=Failed+to+delete+message", http.StatusSeeOther)
		return
	}

	h.log.Infof("deleted message from queue: %s", queueName)
	http.Redirect(w, r, "/queues/"+url.PathEscape(queueName)+"?success=Message+deleted", http.StatusSeeOther)
}

// --- JSON API handlers ---

// handleAPIQueues returns queue list as JSON for auto-refresh.
func (h *handler) handleAPIQueues(w http.ResponseWriter, r *http.Request) {
	queues := h.manager.ListQueues("")
	items := make([]queueListItem, 0, len(queues))
	for _, q := range queues {
		items = append(items, queueListItem{
			Name:      q.Name(),
			IsFifo:    q.IsFifo(),
			Available: q.ApproximateNumberOfMessages(),
			InFlight:  q.ApproximateNumberOfMessagesNotVisible(),
			Delayed:   q.ApproximateNumberOfMessagesDelayed(),
			URL:       q.URL(h.manager.NodeAddress(), h.manager.AccountID()),
		})
	}
	writeJSON(w, items)
}

// handleAPIQueueMessages returns messages for a queue as JSON.
func (h *handler) handleAPIQueueMessages(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/queues/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeJSONError(w, "queue name required", http.StatusBadRequest)
		return
	}

	queueName := parts[0]
	q, err := h.manager.LookupQueue(queueName)
	if err != nil {
		writeJSONError(w, "queue not found", http.StatusNotFound)
		return
	}

	// If sub-path is "messages", return message list
	if len(parts) == 2 && parts[1] == "messages" {
		msgs, _ := q.Store().ReceiveMessages(r.Context(), 10, 1, 0)
		displays := make([]messageDisplay, 0, len(msgs))
		for _, m := range msgs {
			displays = append(displays, messageDisplay{
				MessageID:     m.MessageID,
				ReceiptHandle: m.ReceiptHandle,
				Body:          m.Body,
				ReceiveCount:  m.ApproximateReceiveCount,
				SentTimestamp: m.SentTimestamp.Format("2006-01-02 15:04:05"),
			})
		}
		writeJSON(w, displays)
		return
	}

	// Otherwise return queue summary
	writeJSON(w, queueListItem{
		Name:      q.Name(),
		IsFifo:    q.IsFifo(),
		Available: q.ApproximateNumberOfMessages(),
		InFlight:  q.ApproximateNumberOfMessagesNotVisible(),
		Delayed:   q.ApproximateNumberOfMessagesDelayed(),
		URL:       q.URL(h.manager.NodeAddress(), h.manager.AccountID()),
	})
}

// --- Helpers ---

// renderTemplate executes the named template with the given data.
// It injects MetricsEnabled into the data when the data type supports it.
func (h *handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	// Inject MetricsEnabled into known data types
	switch d := data.(type) {
	case *pageData:
		d.MetricsEnabled = h.metricsEnabled
	case pageData:
		d.MetricsEnabled = h.metricsEnabled
		data = d
	case *queueDetailData:
		d.MetricsEnabled = h.metricsEnabled
	case queueDetailData:
		d.MetricsEnabled = h.metricsEnabled
		data = d
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := templates[name]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// buildAttrPairs converts QueueAttributes into a sorted list of name-value pairs for display.
func buildAttrPairs(attrs *queue.QueueAttributes, q *queue.Queue) []attrPair {
	pairs := []attrPair{
		{"VisibilityTimeout", strconv.Itoa(attrs.VisibilityTimeout)},
		{"MaximumMessageSize", strconv.Itoa(attrs.MaximumMessageSize)},
		{"MessageRetentionPeriod", strconv.Itoa(attrs.MessageRetentionPeriod)},
		{"DelaySeconds", strconv.Itoa(attrs.DelaySeconds)},
		{"ReceiveMessageWaitTimeSeconds", strconv.Itoa(attrs.ReceiveMessageWaitTimeSeconds)},
		{"FifoQueue", strconv.FormatBool(attrs.FifoQueue)},
		{"ContentBasedDeduplication", strconv.FormatBool(attrs.ContentBasedDeduplication)},
		{"QueueArn", attrs.QueueArn},
		{"RedrivePolicy", attrs.RedrivePolicy},
		{"SqsManagedSseEnabled", strconv.FormatBool(attrs.SqsManagedSseEnabled)},
	}
	return pairs
}

// --- Metrics handlers ---

// handleMetrics renders the metrics dashboard page.
func (h *handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.metricsEnabled {
		http.NotFound(w, r)
		return
	}
	data := pageData{
		Title:          "Metrics",
		MetricsEnabled: true,
		Metrics:        collectMetrics(),
	}
	h.renderTemplate(w, "metrics.html", data)
}

// handleAPIMetrics returns parsed metrics as JSON for auto-refresh.
func (h *handler) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.metricsEnabled {
		writeJSONError(w, "metrics not enabled", http.StatusNotFound)
		return
	}
	writeJSON(w, collectMetrics())
}

// collectMetrics gathers Prometheus metrics from the default registry
// and converts them into structured data for display.
func collectMetrics() *metricsData {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return &metricsData{Raw: "Error gathering metrics: " + err.Error()}
	}

	md := &metricsData{
		APIRequests:      []metricAPIRequest{},
		QueueSizes:       []metricQueueSize{},
		MessagesSent:     []metricCounter{},
		MessagesReceived: []metricCounter{},
		MessagesDeleted:  []metricCounter{},
		MoveTaskMessages: []metricMoveTask{},
	}

	for _, mf := range mfs {
		name := mf.GetName()
		for _, m := range mf.GetMetric() {
			labels := labelMap(m)

			switch name {
			case "opensqs_api_requests_total":
				md.APIRequests = append(md.APIRequests, metricAPIRequest{
					Action:   labels["action"],
					Protocol: labels["protocol"],
					Count:    int(m.GetCounter().GetValue()),
				})

			case "opensqs_queue_size":
				md.QueueSizes = append(md.QueueSizes, metricQueueSize{
					Queue: labels["queue"],
					Type:  labels["type"],
					Size:  int(m.GetGauge().GetValue()),
				})

			case "opensqs_messages_sent_total":
				md.MessagesSent = append(md.MessagesSent, metricCounter{
					Queue: labels["queue"],
					Count: int(m.GetCounter().GetValue()),
				})

			case "opensqs_messages_received_total":
				md.MessagesReceived = append(md.MessagesReceived, metricCounter{
					Queue: labels["queue"],
					Count: int(m.GetCounter().GetValue()),
				})

			case "opensqs_messages_deleted_total":
				md.MessagesDeleted = append(md.MessagesDeleted, metricCounter{
					Queue: labels["queue"],
					Count: int(m.GetCounter().GetValue()),
				})

			case "opensqs_move_task_messages_moved_total":
				md.MoveTaskMessages = append(md.MoveTaskMessages, metricMoveTask{
					SourceARN:      labels["source_arn"],
					DestinationARN: labels["destination_arn"],
					Count:          int(m.GetCounter().GetValue()),
				})

			case "opensqs_move_task_active":
				md.MoveTaskActive = int(m.GetGauge().GetValue())
			}
		}
	}

	md.Raw = gatherRaw(mfs)
	return md
}

// labelMap converts a metric's labels into a map for easy lookup.
func labelMap(m *dto.Metric) map[string]string {
	out := make(map[string]string, len(m.GetLabel()))
	for _, lp := range m.GetLabel() {
		out[lp.GetName()] = lp.GetValue()
	}
	return out
}

// gatherRaw produces a text representation of all metrics (Prometheus exposition format).
func gatherRaw(mfs []*dto.MetricFamily) string {
	var sb strings.Builder
	for _, mf := range mfs {
		sb.WriteString(mf.String())
		sb.WriteByte('\n')
	}
	return sb.String()
}
