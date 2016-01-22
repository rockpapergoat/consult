package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/outbrain/consult/misc"
	"gopkg.in/alecthomas/kingpin.v2"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"syscall"
)

var (
	app = kingpin.New("consult", "Query Consul catalog for service")
)

type appOpts struct {
	dcs           []string
	JsonFormat    bool
	serverURL     *url.URL
	ConsulConfigs []*api.Config
}

type Command struct {
	opts *appOpts
}

type sshCommand struct {
	QueryCommand
	user string
}

func main() {
	app.Version("0.0.2")
	opts := &appOpts{}

	app.Flag("dc", "Consul datacenter").StringsVar(&opts.dcs)
	app.Flag("server", "Consul URL; can also be provided using the CONSUL_URL environment variable").Default("http://127.0.0.1:8500/").Envar("CONSUL_URL").URLVar(&opts.serverURL)
	app.Flag("json", "JSON query output").Short('j').BoolVar(&opts.JsonFormat)
	app.HelpFlag.Short('h')

	listRegisterCli(app, opts)
	httpRegisterCli(app, opts)
	sshRegisterCli(app, opts)
	queryRegisterCli(app, opts)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func (q *QueryCommand) registerCli(cmd *kingpin.CmdClause) {
	cmd.Flag("tag", "Consul tag").Short('t').StringsVar(&q.tags)
	cmd.Flag("service", "Consul service").Required().Short('s').StringVar(&q.service)
	cmd.Flag("tags-mode", "Find nodes with *all* or *any* of the tags").Short('m').Default("all").EnumVar(&q.tagsMerge, "all", "any")
}

func sshRegisterCli(app *kingpin.Application, opts *appOpts) {
	s := &sshCommand{}
	s.IQuery = s
	s.opts = opts
	sshCmd := app.Command("ssh", "ssh into server using Consul query").Action(s.run)
	sshCmd.Flag("username", "ssh user name").Short('u').StringVar(&s.user)
	s.registerCli(sshCmd)
}

func (s *sshCommand) run(c *kingpin.ParseContext) error {
	results, err := s.queryServicesGeneric()
	if err != nil {
		return err
	}
	ssh(selectRandomSvc(results).Node, s.user)
	return nil
}

func printJsonResults(results []*api.CatalogService) {
	if b, err := json.MarshalIndent(results, "", "    "); err != nil {
		kingpin.Fatalf("Failed to convert results to json, %s\n", err.Error())
	} else {
		fmt.Println(string(b))
	}
}

func selectRandomSvc(services []*api.CatalogService) *api.CatalogService {
	return services[rand.Intn(len(services))]
}

func ssh(address string, user string) {
	bin, err := exec.LookPath("ssh")
	if err != nil {
		kingpin.Fatalf("Failed to find ssh binary: %s\n", err.Error())
	}

	ssh_args := make([]string, 2, 3)
	ssh_args[0] = "ssh"
	ssh_args[1] = address
	if user != "" {
		ssh_args = append(ssh_args, "-l "+user)
	}

	syscall.Exec(bin, ssh_args, os.Environ())
}

func (o *Command) GetConsulClient() (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = o.opts.serverURL.Host
	config.Scheme = o.opts.serverURL.Scheme
	return api.NewClient(config)
}

func (o *Command) GetConsulClients() (map[string]*api.Client, error) {
	clients := make(map[string]*api.Client, len(o.opts.dcs))

	if len(o.opts.dcs) == 0 {
		if client, err := o.GetConsulClient(); err != nil {
			return nil, err
		} else {
			clients[""] = client
			return clients, nil
		}
	}

	for _, dc := range o.opts.dcs {
		config := api.DefaultConfig()
		config.Address = o.opts.serverURL.Host
		config.Scheme = o.opts.serverURL.Scheme
		config.Datacenter = dc

		if client, err := api.NewClient(config); err != nil {
			return nil, err
		} else {
			clients[dc] = client
		}
	}
	return clients, nil
}

func (o *Command) Output(data interface{}) {
	if o.opts.JsonFormat {
		if b, err := json.MarshalIndent(data, "", "    "); err != nil {
			kingpin.Fatalf("Failed to convert results to json, %s\n", err.Error())
		} else {
			fmt.Println(string(b))
		}
	} else {
		misc.PrettyPrint(data)
	}
}
