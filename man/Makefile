BOOK := ../book/src
SHELL := /bin/bash

# Detect sed variant (macOS uses gsed from Homebrew)
SED := $(shell command -v gsed 2>/dev/null || echo sed)

.PHONY: all man clean

# Default: do everything
all: man

# Step 1: assemble the prompt
prompt: base-prompt \
            $(BOOK)/command/super.md \
            $(BOOK)/command/options.md \
            $(BOOK)/command/input.md \
            $(BOOK)/command/output.md \
            $(BOOK)/command/formats.md \
            $(BOOK)/getting-started/tldr.md \
            $(BOOK)/command/db.md \
            $(BOOK)/command/compile.md \
            $(BOOK)/command/dev.md
	@echo "Assembling prompt..."
	@cp base-prompt prompt
	@echo -e "\nThe \"super -h\" output:\n==============================" >> prompt
	@super -h | $(SED) 's/\x1b\[[0-9;]*m//g' >> prompt
	@for file in super.md options.md input.md output.md formats.md; do \
	  echo -e "\nContents of \"$$file\" from detailed mdbook documentation:" >> prompt; \
	  echo "================================================================" >> prompt; \
	  cat $(BOOK)/command/$$file >> prompt; \
	done
	@echo -e "\nSource material from detailed mdbook documentation for EXAMPLES in manpage:" >> prompt
	@echo "================================================================" >> prompt
	@cat $(BOOK)/getting-started/tldr.md >> prompt
	@for file in db.md compile.md dev.md; do \
	  echo -e "\nFirst 50 lines of \"$$file\" from detailed mdbook documentation:" >> prompt; \
	  echo "================================================================" >> prompt; \
	  head -50 $(BOOK)/command/$$file >> prompt; \
	done
	@$(SED) -i "s/DATE_PLACEHOLDER/$$(date '+%B %Y')/" prompt
	@echo "Done. prompt is ready."

# Step 2: call Claude API to generate super.1.md
super.1.md: prompt
	@if [ -z "$$ANTHROPIC_API_KEY" ]; then \
	  echo "ANTHROPIC_API_KEY is not set."; \
	  echo "Submit prompt to Claude manually, save the result as super.1.md,"; \
	  echo "then run: make man"; \
	  exit 0; \
	fi
	@echo "Calling Claude API..."
	@jq -n --rawfile content prompt '{ \
	  model: "claude-sonnet-4-6", \
	  max_tokens: 4096, \
	  messages: [{ role: "user", content: $$content }] \
	}' | curl -s https://api.anthropic.com/v1/messages \
	  --header "x-api-key: $$ANTHROPIC_API_KEY" \
	  --header "anthropic-version: 2023-06-01" \
	  --header "content-type: application/json" \
	  --data @- \
	  | jq -r '.content[0].text' > super.1.md
	@echo "Done. super.1.md is ready."

# Step 3: run pandoc
man: super.1.md
	@echo "Generating super.1..."
	@pandoc super.1.md -s -t man -o ../man1/super.1
	@echo "Done. ../man1/super.1 is ready."

clean:
	@rm -f prompt super.1.md ../man1/super.1
	@echo "Cleaned."
