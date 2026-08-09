"""
This module provides a rule to write content to a file with optional substitutions.
"""

def _write_content_impl(ctx):
    template = ctx.actions.declare_file("{}.template".format(ctx.label.name))
    ctx.actions.write(
        output = template,
        content = ctx.attr.content,
    )

    ctx.actions.expand_template(
        template = template,
        output = ctx.outputs.content,
        substitutions = ctx.attr.substitutions,
        is_executable = ctx.attr.executable,
    )

write_content = rule(
    implementation = _write_content_impl,
    doc = """
        Write some content to a file, making some optional substitutions.
    """,
    attrs = {
        "content": attr.string(
            doc = "Arbitrary content to write to a file in the form of a string",
            default = "",
        ),
        "script_file_name": attr.string(
            doc = "Name of the output file",
        ),
        "executable": attr.bool(
            doc = "If true, set the created file to be executable",
            default = False,
        ),
        "substitutions": attr.string_dict(
            doc = "Key-value substitutions to be made in the content",
            default = {},
        ),
    },
    outputs = {
        # "content": "%{name}.content",
        "content": "%{script_file_name}",
    },
)
