#!/bin/bash
# Verify every generated doc under docs/{resources,data-sources} has a matching
# template under templates/{resources,data-sources}, and vice-versa.
#
# tfplugindocs silently falls back to a built-in template when one is missing,
# producing a page with the wrong `subcategory`/`page_title` (it still succeeds,
# so the gap is easy to miss). This guard fails loudly when a resource or data
# source is added without its template (or a template is left behind).
#
# Usage: ./scripts/check-docs-templates.sh

set -euo pipefail

status=0

for kind in resources data-sources; do
    docs_dir="docs/${kind}"
    tmpl_dir="templates/${kind}"

    # Generated docs that have no template.
    if [ -d "$docs_dir" ]; then
        for doc in "$docs_dir"/*.md; do
            [ -e "$doc" ] || continue
            name="$(basename "$doc" .md)"
            if [ ! -f "${tmpl_dir}/${name}.md.tmpl" ]; then
                echo "ERROR: ${doc} has no template (expected ${tmpl_dir}/${name}.md.tmpl)"
                status=1
            fi
        done
    fi

    # Templates that have no corresponding generated doc (orphans).
    if [ -d "$tmpl_dir" ]; then
        for tmpl in "$tmpl_dir"/*.md.tmpl; do
            [ -e "$tmpl" ] || continue
            name="$(basename "$tmpl" .md.tmpl)"
            if [ ! -f "${docs_dir}/${name}.md" ]; then
                echo "ERROR: ${tmpl} has no generated doc (expected ${docs_dir}/${name}.md)"
                status=1
            fi
        done
    fi
done

if [ "$status" -ne 0 ]; then
    echo ""
    echo "Documentation/template mismatch. For a missing template, copy an existing"
    echo "templates/<kind>/*.md.tmpl (set the right subcategory) and run 'make docs';"
    echo "for an orphan template, remove it."
    exit 1
fi

echo "OK: every resource/data-source doc has a matching template (and vice-versa)."
