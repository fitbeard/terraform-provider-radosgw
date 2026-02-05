#!/bin/bash
# Transform tfplugindocs Schema format to traditional Argument Reference format
# Usage: ./scripts/transform-docs.sh [docs_dir]

set -e

DOCS_DIR="${1:-docs}"

transform_file() {
    local file="$1"
    local temp_file=$(mktemp)

    awk '
    BEGIN {
        in_schema = 0
        in_required = 0
        in_optional = 0
        in_readonly = 0
        in_nested = 0
        printed_arg_header = 0
        printed_attr_header = 0
        arg_count = 0
        readonly_count = 0
    }

    # Detect Schema section
    /^## Schema$/ {
        in_schema = 1
        next
    }

    # Detect subsections within Schema
    /^### Required$/ {
        if (in_schema) {
            in_required = 1
            in_optional = 0
            in_readonly = 0
            if (!printed_arg_header) {
                print "## Argument Reference"
                print ""
                print "The following arguments are supported:"
                print ""
                printed_arg_header = 1
            }
            next
        }
    }

    /^### Optional$/ {
        if (in_schema) {
            in_required = 0
            in_optional = 1
            in_readonly = 0
            if (!printed_arg_header) {
                print "## Argument Reference"
                print ""
                print "The following arguments are supported:"
                print ""
                printed_arg_header = 1
            }
            next
        }
    }

    /^### Read-Only$/ {
        if (in_schema) {
            in_required = 0
            in_optional = 0
            in_readonly = 1
            # Do not print header yet - we will print it with all attributes later
            next
        }
    }

    # Handle nested schema anchor tags
    /^<a id=.*><\/a>$/ {
        if (in_schema) {
            # Before starting nested schema, print Attributes Reference if we have args
            if (!in_nested && !printed_attr_header) {
                print ""
                print "## Attributes Reference"
                print ""
                print "The following attributes are exported:"
                print ""
                # Print read-only attributes first
                for (i = 0; i < readonly_count; i++) {
                    print readonly_attrs[i]
                }
                # Then print argument references
                for (i = 0; i < arg_count; i++) {
                    printf "* `%s` - See Argument Reference above.\n", arg_names[i]
                }
                printed_attr_header = 1
            }
            in_nested = 1
            print ""
            print $0
            next
        }
    }

    # Handle nested schema headers
    /^### Nested Schema for/ {
        if (in_schema) {
            # Before starting nested schema, print Attributes Reference if we have args
            if (!in_nested && !printed_attr_header) {
                print ""
                print "## Attributes Reference"
                print ""
                print "The following attributes are exported:"
                print ""
                # Print read-only attributes first
                for (i = 0; i < readonly_count; i++) {
                    print readonly_attrs[i]
                }
                # Then print argument references
                for (i = 0; i < arg_count; i++) {
                    printf "* `%s` - See Argument Reference above.\n", arg_names[i]
                }
                printed_attr_header = 1
            }
            in_nested = 1
            print $0
            next
        }
    }

    # End of schema section (next major section like ## Import)
    /^## / {
        if (in_schema && !/^## Argument/ && !/^## Attributes/) {
            # Print Attributes Reference before ending schema section
            if (!printed_attr_header) {
                print ""
                print "## Attributes Reference"
                print ""
                print "The following attributes are exported:"
                print ""
                # Print read-only attributes first
                for (i = 0; i < readonly_count; i++) {
                    print readonly_attrs[i]
                }
                # Then print argument references
                for (i = 0; i < arg_count; i++) {
                    printf "* `%s` - See Argument Reference above.\n", arg_names[i]
                }
                printed_attr_header = 1
            }
            in_schema = 0
            in_required = 0
            in_optional = 0
            in_readonly = 0
            in_nested = 0
        }
        print
        next
    }

    # Transform attribute lines: - `name` (Type) Description
    /^- `[^`]+` \([^)]+\)/ {
        if (in_schema && !in_nested) {
            # Remove leading "- "
            line = substr($0, 3)

            # Find the attribute name between backticks
            start = index(line, "`")
            end = index(substr(line, start+1), "`")
            name = substr(line, start+1, end-1)

            # Find the type in parentheses
            rest = substr(line, start+end+2)
            type_start = index(rest, "(")
            type_end = index(rest, ")")
            type_val = substr(rest, type_start+1, type_end-type_start-1)

            # Get description after the type
            desc = substr(rest, type_end+2)

            if (in_required) {
                printf "* `%s` - (Required) %s\n", name, desc
                arg_names[arg_count] = name
                arg_count++
            } else if (in_optional) {
                printf "* `%s` - (Optional) %s\n", name, desc
                arg_names[arg_count] = name
                arg_count++
            } else if (in_readonly) {
                # Store read-only attributes for later
                readonly_attrs[readonly_count] = sprintf("* `%s` - %s", name, desc)
                readonly_count++
            }
            next
        }
    }

    # Handle nested schema section markers
    /^Optional:$/ {
        if (in_schema && in_nested) {
            print ""
            next
        }
    }

    /^Read-Only:$/ {
        if (in_schema && in_nested) {
            print ""
            next
        }
    }

    # At end of file, make sure we printed attributes if needed
    END {
        if (in_schema && !printed_attr_header && (readonly_count > 0 || arg_count > 0)) {
            print ""
            print "## Attributes Reference"
            print ""
            print "The following attributes are exported:"
            print ""
            for (i = 0; i < readonly_count; i++) {
                print readonly_attrs[i]
            }
            for (i = 0; i < arg_count; i++) {
                printf "* `%s` - See Argument Reference above.\n", arg_names[i]
            }
        }
    }

    # Print all other lines
    {
        print
    }
    ' "$file" > "$temp_file"

    mv "$temp_file" "$file"
}

echo "Transforming documentation format..."

# Transform all resource and data source docs
for file in "$DOCS_DIR"/resources/*.md "$DOCS_DIR"/data-sources/*.md; do
    if [[ -f "$file" ]]; then
        echo "  Processing: $(basename "$file")"
        transform_file "$file"
    fi
done

echo "Documentation transformation complete."
