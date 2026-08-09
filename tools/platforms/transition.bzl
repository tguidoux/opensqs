"""
Multi-architecture image transition.

Wraps oci_image_index with platform transitions so that a single target
produces a multi-arch manifest.
"""

load("@bazel_lib//lib:transitions.bzl", "platform_transition_filegroup")
load("@rules_oci//oci:defs.bzl", "oci_image_index")

def multi_arch(name, image, platforms, **kwargs):
    """
    Create a multi-architecture OCI image index.

    For each platform, a platform_transition_filegroup is created that
    transitions the image to the target platform. All platform-specific
    images are then combined into a single oci_image_index.

    Args:
        name: A unique name for this target.
        image: The label of the base oci_image to transition.
        platforms: A list of platform labels to build for.
        **kwargs: Additional arguments passed to oci_image_index.
    """
    platform_images = []

    for idx, plat in enumerate(platforms):
        plat_name = plat.split(":")[-1]
        transition_name = "{}_{}".format(name, plat_name)

        platform_transition_filegroup(
            name = transition_name,
            srcs = [image],
            target_platform = plat,
        )

        platform_images.append(":{}".format(transition_name))

    oci_image_index(
        name = name,
        images = platform_images,
        **kwargs
    )
