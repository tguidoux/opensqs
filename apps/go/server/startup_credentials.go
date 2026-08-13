package main

import (
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/credentials"
)

// seedInitialCredentials imports pre-configured credentials into the store
// at startup. This allows users to bring their own AWS-style credentials
// (e.g. from an external identity provider) and use them with OpenSQS
// from the first boot, without having to create credentials via the UI.
//
// If a credential with the same accessKeyId already exists in the store,
// the import fails with a fatal error (no silent skip, no upsert).
//
// This function is a no-op when auth is disabled or no initial credentials
// are configured.
func seedInitialCredentials(
	credStore credentials.CredentialStore,
	cfg AuthConfig,
	defaultRegion, defaultAccountID string,
	log logger.LoggerInterface,
) {
	if len(cfg.InitialCredentials) == 0 {
		return
	}

	log.Infof("seeding %d initial credential(s) from config", len(cfg.InitialCredentials))

	for _, ic := range cfg.InitialCredentials {
		label := ic.Label
		if label == "" {
			label = "imported"
		}
		region := ic.Region
		if region == "" {
			region = defaultRegion
		}
		accountID := ic.AccountID
		if accountID == "" {
			accountID = defaultAccountID
		}

		// Check if the credential already exists (e.g. after a restart
		// with persistent storage). Skip import if it does — this is
		// not an error, the credential was seeded on a previous boot.
		if existing, err := credStore.GetByAccessKeyID(ic.AccessKeyID); err == nil && existing != nil {
			log.Infof("initial credential %q already exists (accessKeyId: %s), skipping import", label, ic.AccessKeyID)
			continue
		}

		_, err := credStore.Import(label, ic.AccessKeyID, ic.SecretAccessKey, region, accountID)
		if err != nil {
			log.Fatalf("failed to import initial credential %q: %v", label, err)
			return
		}

		log.Infof("imported initial credential %q (accessKeyId: %s)", label, ic.AccessKeyID)
	}
}
