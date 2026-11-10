BIN       := s3reader
PREFIX    ?= $(HOME)/.local
BINDIR    ?= $(PREFIX)/bin
CONFIGDIR ?= $(or $(XDG_CONFIG_HOME),$(HOME)/.config)/s3reader

.PHONY: build install config uninstall test vet clean

build:
	go build -trimpath -o $(BIN) .

## install: build and copy the binary to $(BINDIR) (override with PREFIX=/usr/local)
install: build
	install -d $(BINDIR)
	install -m 0755 $(BIN) $(BINDIR)/$(BIN)
	@echo "installed $(BINDIR)/$(BIN)"
	@case ":$$PATH:" in *":$(BINDIR):"*) ;; *) echo "note: $(BINDIR) is not in PATH";; esac

## config: create $(CONFIGDIR)/settings.json from the example, never overwriting
config:
	@install -d -m 0700 $(CONFIGDIR)
	@if [ -e $(CONFIGDIR)/settings.json ]; then \
		echo "$(CONFIGDIR)/settings.json exists, left unchanged"; \
	else \
		install -m 0600 settings.example.json $(CONFIGDIR)/settings.json; \
		echo "created $(CONFIGDIR)/settings.json, edit it to add your profiles"; \
	fi

uninstall:
	rm -f $(BINDIR)/$(BIN)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BIN)
