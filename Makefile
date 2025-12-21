PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
GO ?= go

all: grc

grc:
	$(GO) build -o grc ./cmd/grc

test:
	$(GO) test ./...

check: test

install: grc
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 grc $(DESTDIR)$(BINDIR)/grc

clean:
	rm -f grc

distclean: clean

.PHONY: all grc test check install clean distclean
