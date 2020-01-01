/*
  Copyright 2020 Star Brilliant <coder@poorlab.com>

  Permission is hereby granted, free of charge, to any person obtaining a copy
  of this software and associated documentation files (the "Software"), to deal
  in the Software without restriction, including without limitation the rights
  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
  copies of the Software, and to permit persons to whom the Software is
  furnished to do so, subject to the following conditions:

  The above copyright notice and this permission notice shall be included in
  all copies or substantial portions of the Software.

  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
  SOFTWARE.
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/gorilla/handlers"
)

type config struct {
	HTTPAddr   string `toml:"http_addr"`
	HTTPPath   string `toml:"http_path"`
	Token      string `toml:"token"`
	Zone       string `toml:"zone"`
	RecordA    string `toml:"record_a"`
	RecordAAAA string `toml:"record_aaaa"`
}

type updateHandler struct {
	Config *config
}

func (h *updateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.Header().Set("Server", "mikrotik-cf-ddns/1.0 (+https://github.com/GreenYun/mikrotik-cf-ddns)")

	if r.Method != "POST" {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	bodyText := strings.TrimSpace(string(body))

	if strings.IndexByte(bodyText, ':') == -1 {
		if h.Config.RecordA == "" {
			log.Println("IPv4 address support is not enabled")
			http.Error(w, "IPv4 address support is not enabled", http.StatusBadRequest)
			return
		}
		h.updateRecord(w, r, h.Config.Zone, h.Config.RecordA, bodyText)
		return
	}

	if h.Config.RecordAAAA == "" {
		log.Println("IPv6 address support is not enabled")
		http.Error(w, "IPv6 address support is not enabled", http.StatusBadRequest)
		return
	}
	h.updateRecord(w, r, h.Config.Zone, h.Config.RecordAAAA, bodyText)
}

func (h *updateHandler) updateRecord(w http.ResponseWriter, r *http.Request, zoneID, recordID, value string) {
	reqBodyText := fmt.Sprintf("{\"content\":\"%s\"}", template.JSEscapeString(value))
	reqBody := strings.NewReader(reqBodyText)
	req, err := http.NewRequestWithContext(r.Context(), "PATCH", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recordID)), reqBody)
	if err != nil {
		log.Printf("cannot connect to upstream server: %v\n", err)
		http.Error(w, "cannot connect to upstream server", http.StatusBadGateway)
		return
	}

	req.Header.Set("Authorization", "Bearer "+h.Config.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("cannot connect to upstream server: %v\n", err)
		http.Error(w, "cannot connect to upstream server", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if value := resp.Header.Get("Content-Length"); value != "" {
		w.Header().Set("Content-Length", value)
	}
	if value := resp.Header.Get("Content-Type"); value != "" {
		w.Header().Set("Content-Type", value)
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func parseConfig(path string) (*config, error) {
	conf := new(config)
	metaData, err := toml.DecodeFile(path, conf)
	if err != nil {
		return nil, err
	}

	for _, key := range metaData.Undecoded() {
		panic(fmt.Errorf("unknown option %q", key.String()))
	}

	if conf.HTTPAddr == "" {
		conf.HTTPAddr = ":28275"
	}
	if conf.HTTPPath == "" {
		conf.HTTPPath = "/update"
	}
	if conf.Token == "" {
		return nil, errors.New("token is empty")
	}
	if conf.Zone == "" {
		return nil, errors.New("zone is empty")
	}

	return conf, nil
}

func main() {
	path := flag.String("conf", "/etc/mikrotik-cf-ddns.conf", "Configuration file path")
	flag.Parse()

	conf, err := parseConfig(*path)
	if err != nil {
		panic(err)
	}

	servemux := http.NewServeMux()
	servemux.Handle(conf.HTTPPath, &updateHandler{Config: conf})
	logHandler := handlers.CombinedLoggingHandler(os.Stdout, servemux)

	log.Printf("Listening on %s, path %s\n", conf.HTTPAddr, conf.HTTPPath)
	http.ListenAndServe(conf.HTTPAddr, logHandler)
}
