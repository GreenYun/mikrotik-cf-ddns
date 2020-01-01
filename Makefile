.PHONY: all clean

GO=go
RM=rm

all: mikrotik-cf-ddns

clean:
	$(RM) -f mikrotik-cf-ddns

mikrotik-cf-ddns: main.go
	$(GO) get
	$(GO) build -o $@ .
