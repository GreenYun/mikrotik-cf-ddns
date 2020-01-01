.PHONY: all clean

all: mikrotik-cf-ddns

clean:
	rm -f mikrotik-cf-ddns

mikrotik-cf-ddns: main.go
	go get
	go build -o $@ .
