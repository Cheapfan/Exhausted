.PHONY: all build test clean run-noisecomplaint

BINDIR := bin
CMDS := noisecomplaint panicclipper gogopher

all: test build

build:
	@mkdir -p $(BINDIR)
	@for cmd in $(CMDS); do \
		if [ -d "cmd/$$cmd" ]; then \
			echo "  building cmd/$$cmd..."; \
			go build -o $(BINDIR)/$$cmd ./cmd/$$cmd; \
		fi \
	done

test:
	go test ./internal/... -v

clean:
	rm -rf $(BINDIR)

run-noisecomplaint: build
	./$(BINDIR)/noisecomplaint $(ARGS)
