"""
Go rules.
"""

load("@bazel_lib//lib:expand_template.bzl", "expand_template")
load("@bazel_lib//lib:transitions.bzl", "platform_transition_filegroup")
load("@rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_image_index", "oci_load", "oci_push")
load("@tar.bzl", "mtree_spec", "tar")
load("//tools:defs.bzl", "REGISTRY")
load("//tools/platforms:transition.bzl", "multi_arch")

def opensqs_go_test(name, **kwargs):
    """
    A macro that runs golang tests.

    It is a passthrough shim to allow later customization of the go_test rule
    from a single location.

    Args:
        name: The name of the test.
        **kwargs: Additional arguments to pass to go_test.
    """
    go_test(
        name = name,
        **kwargs
    )

def opensqs_go_library(name, **kwargs):
    """
    A macro that creates a golang library.

    It is a passthrough shim to allow later customization of the go_library rule
    from a single location.

    Args:
        name: The name of the library.
        **kwargs: Additional arguments to pass to go_library.
    """
    go_library(
        name = name,
        **kwargs
    )

def opensqs_go_binary(name, auto_load_config = False, **kwargs):
    """
    A macro that creates a golang binary.

    It is a passthrough shim to allow later customization of the go_binary rule
    from a single location.

    Args:
        name: The name of the binary.
        auto_load_config: If True, automatically loads the config.yaml file from the same directory as the BUILD file.
            This is useful for applications that require a configuration file.
            If False, the user must provide a CONFIG_PATH environment variable in kwargs['env'].
        **kwargs: Additional arguments to pass to go_binary.
    """

    if auto_load_config:
        CONFIG_PATH = "$(location :config.yaml)"
        if "env" not in kwargs:
            kwargs["env"] = {}
        if "CONFIG_PATH" not in kwargs["env"]:
            # If the user didn't provide a CONFIG_PATH, we set it to the default.
            # This allows us to use the config.yaml file in the same directory as the BUILD file.
            kwargs["env"]["CONFIG_PATH"] = CONFIG_PATH

    go_binary(
        name = name,
        **kwargs
    )

def _go_layers(name, binary):
    """
    Create the layers for a go_binary target.

    By intelligently bunding layers, we can isolate application changes from other
    layers, which can speed up the build process.

    At the moment, we just provide a single layer for the binary+runfiles, but
    we could improve this in the future similar to how the `opensqs_py_image` macro
    divides up layers.

    Args:
        name: Prefix for generated targets, to ensure they are unique within the package.
        binary: The name of the opensqs_go_binary to bundle in the container.

    Returns:
        A list of labels for the generated layers, which are tar files.
    """

    # The order of the layers here should be from least to most frequently changing.
    layers = ["app"]

    # Produce a manifest for a tar file of our go_binary, but don't tar it up yet. We will split
    # into fine-grained layers later for better docker performance.
    mtree_spec(
        name = name + ".mf",
        srcs = [binary],
    )

    # This all basically works by defining separate regexes for paths that should be included in
    # each layer. The mtree spec is then used to create a tar file for each layer.
    # Since we only bundle an app layer right now, we just use a catch-all regex for it.
    APP_LAYER_REGEX = ".*"

    # Create the tar manifest specs for each layer by taking the base manifest and
    # filtering it with the appropriate regex.
    native.genrule(
        name = name + ".app_tar_manifest",
        srcs = [name + ".mf"],
        outs = [name + ".app_tar_manifest_spec"],
        cmd = "grep '{}' $< >$@".format(APP_LAYER_REGEX),
    )

    result = []
    for layer in layers:
        layer_target = "{}.{}_layer".format(name, layer)
        result.append(layer_target)
        tar(
            name = layer_target,
            srcs = [binary],
            mtree = "{}.{}_tar_manifest".format(name, layer),
        )

    return result

def opensqs_go_image(name, binary, image_tags, tars = [], base = None, entrypoint = None, registry = REGISTRY, **kwargs):
    """
    A macro that generates an OCI container image to run a go_binary target.

    The created target can be passed to anything that expects an oci_iamge target, such as `oci_push`.

    An implicit `oci_tarball` target is created for the image in question, which can be used to load
    this image into a running docker daemon automatically for testing. This is named `name + "_load_docker"`.

        ```sh
        bazel run //path/to:<my_oci_image>_load_docker
        ```

    Args:
        name: A unique name for this target.
        binary: The name of the opensqs_go_binary to bundle in the container.
        image_tags: A list of tags to apply to the image.
        tars: A list of additional tar files to include in the image.
        base: The base image to use for the container. If not provided, the default is "@distroless_base".
        entrypoint: The entrypoint for the container. If not provided, it is inferred from the binary.
        registry: The container registry to push the image to. If not provided, defaults to REGISTRY.
        **kwargs: are passed to oci_image

    Example:
        opensqs_go_image(
            name = "my_image",
            binary = "//path/to:my_py_binary",
            tars = ["//path/to:my_extra_tar"],
            base = "@distroless_base",
            entrypoint = ["/my_binary"],
            image_tags = ["my-tag:latest"],
        )
    """
    base = base or "@ubuntu_base"

    # If the user didn't provide an entrypoint, we can infer it from the binary.
    bin_name = binary.split(":")[1]
    workspace_path = ""
    if binary.startswith("//"):
        workspace_path = binary.split(":")[0][2:]
    else:
        workspace_path = native.package_name()

    # Outputs from go_binary targets add an extra directory with a random underscore to the path
    entrypoint = entrypoint or ["/{}/{}_/{}".format(workspace_path, bin_name, bin_name)]

    # extract the image name and tags from the provided image_tags
    names = []
    tags = []
    for tag in image_tags:
        if ":" in tag:
            image_name, image_tag = tag.split(":", 1)
        else:
            image_name = tag
            image_tag = "latest"
        names.append(image_name)
        tags.append(image_tag)

    if len(names) == 0:
        fail("At least one image tag must be provided in `image_tags`")

    # Define the image we want to provide
    oci_image(
        name = name + "_base_img",
        tars = tars + _go_layers(name, binary),
        base = base,
        entrypoint = entrypoint,
        **kwargs
    )

    # Create a load target for the base image
    oci_load(
        name = "{}_base_img_load_docker".format(name),
        image = name + "_base_img",
        repo_tags = image_tags,
    )

    # Transition the image to the platform we're building for
    platform_transition_filegroup(
        name = "{}_platform_transition".format(name),
        srcs = ["{}_base_img".format(name)],
        target_platform = select({
            "@platforms//cpu:arm64": "//tools/platforms:linux_arm64",
            "@platforms//cpu:x86_64": "//tools/platforms:linux_amd64",
        }),
    )

    # Create a load target for the platform transition image
    oci_load(
        name = "{}_platform_transition_load_docker".format(name),
        image = ":{}_platform_transition".format(name),
        repo_tags = image_tags,
    )

    multi_arch(
        name = "{}_multi_arch".format(name),
        image = ":{}_platform_transition".format(name),
        platforms = [
            "//tools/platforms:linux_arm64",
            "//tools/platforms:linux_amd64",
        ],
        visibility = ["//visibility:public"],
    )

    oci_image_index(
        name = name,
        images = [
            ":{}_multi_arch".format(name),
        ],
        visibility = ["//visibility:public"],
    )

    expand_template(
        name = "{}_stamped".format(name),
        out = "{}_stamped.tags.txt".format(name),
        stamp_substitutions = {
            t: "{{BUILD_EMBED_LABEL}}"
            for t in tags
        },
        template = tags,
    )

    oci_push(
        name = "{}_push".format(name),
        image = name,
        remote_tags = ":{}_stamped".format(name),
        repository = "{}/{}".format(registry, names[0]),
    )

    # Note: oci_load doesn't support multi-arch image indices
    # Use the platform-specific load targets instead:
    # - {}_base_img_load_docker
    # - {}_platform_transition_load_docker
