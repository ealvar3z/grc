PREFIX ?= /usr/local
DEV ?= /home/eax/.go
DEVDIR ?= $(DEV)/bin
BINDIR ?= $(PREFIX)/bin
GO ?= go

all: grc

grc:
	$(GO) build -o grc ./cmd/grc

test:
	$(GO) test ./...

check: test

dev: grc
	install -d $(DEV)$(DEVDIR)
	install -m 755 grc $(DEV)$(DEVDIR)/grc

install: grc
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 grc $(DESTDIR)$(BINDIR)/grc

clean:
	rm -f grc

distclean: clean

.PHONY: all grc test check dev install clean distclean
