def npm_link_all_packages(name):
    native.alias(
        name = name,
        actual = "//pkg/ui:empty_node_modules",
    )
