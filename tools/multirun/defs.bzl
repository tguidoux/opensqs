"""
This module provides functions to run multiple bazel invocations in a single
task.
"""

load("//tools/write_content:defs.bzl", "write_content")

DEFAULT_BEFORE_SCRIPT = []

def multi_test(**kwargs):
    multi(action = "test", **kwargs)

def multi_run(**kwargs):
    multi(action = "run", **kwargs)

def multi_build(**kwargs):
    multi(action = "build", **kwargs)

def multi(name, action, targets, startup_flags = [], extra_flags = [], before_script = [], after_script = []):
    """
    Run a Bazel action in the Bazel CI image.

    Note: This rule assumes that the `@bazel_image` docker image has already
    been pulled

    Args:
        name: name of the task
        action: bazel command
        targets: list of bazel targets, if action is run then this task will
            create a seperate Bazel invocation for each target
        startup_flags: bazel startup flags
        extra_flags: standard bazel action flags
        before_script: list of commands to run before bazel invocations
        after_script: list of commands to run after bazel invocations
    """

    script = ["set -euxo pipefail"]

    template = "bazel {startup_flags} {action} {extra_flags} -- {targets}"

    startup_flags_str = " ".join(startup_flags)
    extra_flags_str = " ".join(extra_flags)

    if action == "run":
        for t in targets:
            script.append(template.format(action = action, targets = t, extra_flags = extra_flags_str, startup_flags = startup_flags_str))
    else:
        script.append(template.format(action = action, targets = " ".join(targets), extra_flags = extra_flags_str, startup_flags = startup_flags_str))

    write_content(
        name = "{name}.script".format(name = name),
        script_file_name = name,
        content = "\n".join(before_script + script + after_script),
        executable = True,
    )
