# Formatting for the Go code and the C and Objective-C cgo bridges.
# clang-format reads its rules from .clang-format.

CLANG_FORMAT ?= clang-format

# find, not git ls-files: it includes new files and skips deleted ones.
C_FILES := $(shell find . -path ./.git -prune -o -type f \( -name '*.c' -o -name '*.h' -o -name '*.m' \) -print)

.PHONY: fmt fmt-check

# fmt formats all Go, C, and Objective-C files in place.
fmt:
	gofmt -w .
	$(CLANG_FORMAT) -i $(C_FILES)

# fmt-check fails if a file is not formatted. It changes no files.
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt'd:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	$(CLANG_FORMAT) --dry-run --Werror $(C_FILES)
