package main

import (
	"crypto/tls"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
        "github.com/go-kit/kit/log/level"

        "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	clusterList = make(map[string]string)
	hostList    = make(map[string]string)
)

var (
        clusterMetrics = make(map[string]*prometheus.GaugeVec)
        hostMetrics    = make(map[string]*prometheus.GaugeVec)
)

type Clusters struct {
	Clusters []Cluster `xml:"cluster"`
}

type Cluster struct {
	Id string `xml:"id,attr"`
	Name string `xml:"name"`
}

type Hosts struct {
	Hosts []Host `xml:"host"`
}

type Host struct {
	Id string `xml:"id,attr"`
	Name string `xml:"name"`
	Status Status `xml:"status"`
}

type Status struct {
	State string `xml:"state"`
}

type Stats struct {
	Stats []Stat `xml:"statistic"`
}

type Stat struct {
	Name string `xml:"name"`
	Description string `xml:"description"`
	Type string `xml:"type"`
	Unit string `xml:"unit"`
	Values Values `xml:"values"`
}

type Values struct {
	Value Value `xml:"value"`
}

type Value struct {
	Datum float64 `xml:"datum"`
}


func call(base string, uri string, user string, password string, v interface{}) error {

        tlsConf := &tls.Config{InsecureSkipVerify: true}
        transport := &http.Transport{TLSClientConfig: tlsConf}
        timeout := time.Duration(30 * time.Second)
        client := http.Client{
                Transport: transport,
                Timeout: timeout,
        }

	request, err := http.NewRequest("GET", base + uri, nil)
	if err != nil {
		return err
	}

        request.Header.Set("Accept", "application/xml")
        request.SetBasicAuth(user, password)

        response, err := client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err := xml.Unmarshal(body, &v); err != nil {
		return err
	}

	return nil
}

func initMetrics(config config, logger log.Logger) {
	level.Info(logger).Log("msg", "initalizing metrics")

	// haal cluster info op
		// map cluster id en naam
	// haal host info op
		// map host id en naam
	// haal metrics voor 1 host op


	var clusters Clusters
	if err := call(config.endpoint, "/clusters", config.user, config.password, &clusters); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/clusters", "error", err)
	}
	for _, cluster := range clusters.Clusters {
	       clusterList[cluster.Id] = cluster.Name
	}

	var hosts Hosts
	if err := call(config.endpoint, "/hosts", config.user, config.password, &hosts); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/hosts","error", err)
	}
	for _, host := range hosts.Hosts {
		hostList[host.Id] = host.Name
	}

	var stats Stats
	if err := call(config.endpoint, "/hosts/" + hosts.Hosts[0].Id + "/statistics", config.user, config.password, &stats); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/hosts/" + hosts.Hosts[0].Id + "/statistics" ,"error", err)
	}
	for _, stat := range stats.Stats {

		level.Debug(logger).Log("msg", "registering stat", "stat", stat.Name)

		clusterMetrics[stat.Name] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_cluster_" + strings.ReplaceAll(stat.Name, ".", "_"),
                        Help: stat.Description + " in cluster " + stat.Unit,
                },[]string{"cluster"})

		hostMetrics["UsedMem"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_host_" + strings.ReplaceAll(stat.Name, ".", "_"),
                        Help: stat.Description + " on host" + stat.Unit,
                },[]string{"cluster", "host"})
	}

	clusterMetrics["activeHosts"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_cluster_activeHosts",
                        Help: "number of active hosts in cluster ",
                },[]string{"cluster"})

	hostMetrics["up"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_host_up",
                        Help: "is rhev host active",
                },[]string{"cluster", "host"})
}

func recordMetrics(config config, logger log.Logger) {

	for {
		level.Debug(logger).Log("msg", "metrics recording cycle")

		// haal /hosts op en tel active hosts

		// haal metrics per host op

		time.Sleep(time.Duration(config.interval) * time.Second)
	}
}
