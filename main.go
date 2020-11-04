package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

const (
	nameTag    = "dashboard.service.name"
	addressTag = "dashboard.service.address"
)

type service struct {
	Name    string
	Address string
}

var (
	servicesMu sync.RWMutex
	services   = map[string]*service{}
)

const templateText = `
<html>
<head><title>Dashboard</title></head>
<body>
{{range .Services}}
<a href="{{.Address}}">{{.Name}}</a>
{{end}}
</body>
`

func main() {
	httpAddr := flag.String("http-addr", func() string {
		addr := os.Getenv("HTTP_ADDR")
		if addr == "" {
			addr = ":1234"
		}
		return addr
	}(), "Sets the HTTP address to listen on")
	flag.Parse()

	tmpl := template.Must(template.New("dashboard").Parse(templateText))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		servicesMu.RLock()
		svcs := make([]*service, 0, len(services))
		for _, svc := range services {
			svcs = append(svcs, svc)
		}
		servicesMu.RUnlock()

		sort.Slice(svcs, func(i, j int) bool {
			return svcs[i].Name < svcs[j].Name
		})

		_ = tmpl.Execute(w, struct{ Services []*service }{svcs})
	})

	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatalf("consul.NewClient: %v", err)
	}
	go listServices(client)

	_ = http.ListenAndServe(*httpAddr, nil)
}

func listServices(client *api.Client) {
	catalog := client.Catalog()
	var lastIndex uint64

	for {
		opts := &api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  30 * time.Second,
		}

		svcs, meta, err := catalog.Services(opts)
		if err != nil {
			log.Printf("catalog.Services: %v", err)
			time.Sleep(15 * time.Second)
			continue
		}

		validServices := make(map[string]struct{})
		for svc, tags := range svcs {
			name, addr := extractTags(tags)
			if name == "" || addr == "" {
				continue
			}

			servicesMu.Lock()
			if _, ok := services[svc]; !ok {
				services[svc] = &service{}
			}
			services[svc].Name = name
			services[svc].Address = addr
			servicesMu.Unlock()

			validServices[svc] = struct{}{}
		}

		servicesMu.Lock()
		for svc := range services {
			if _, ok := validServices[svc]; !ok {
				delete(services, svc)
			}
		}
		servicesMu.Unlock()

		lastIndex = meta.LastIndex
	}
}

func extractTags(tags []string) (name string, addr string) {
	for _, tag := range tags {
		parts := strings.Split(tag, "=")
		if len(parts) == 2 {
			switch parts[0] {
			case nameTag:
				name = parts[1]
			case addressTag:
				addr = parts[1]
			}
		}
	}

	return name, addr
}
