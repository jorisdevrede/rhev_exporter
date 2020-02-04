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
	hostMetrics = make(map[string]*prometheus.GaugeVec)
)

// calls RHEV REST API and translates xml response to a struct
func call(base string, uri string, user string, password string, v interface{}, setDetail bool) error {

	tlsConf := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{TLSClientConfig: tlsConf}
	timeout := time.Duration(30 * time.Second)
	client := http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	request, err := http.NewRequest("GET", base+uri, nil)
	if err != nil {
		return err
	}

	var detail string
	if setDetail {
		detail = "; detail=statistics"
	}
	request.Header.Set("Accept", "application/xml"+detail)
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

// initializes metrics for export
func initMetrics(config config, logger log.Logger) {

	level.Info(logger).Log("msg", "initalizing metrics")

	// retrieve cluster Names and Ids (8s call)
	var clusters Clusters
	if err := call(config.endpoint, "/clusters", config.user, config.password, &clusters, false); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/clusters", "error", err)
	}
	for _, cluster := range clusters.Clusters {
		clusterList[cluster.Id] = cluster.Name
	}

	// retrieve hosts with stats (8s call)
	var hosts Hosts
	if err := call(config.endpoint, "/hosts", config.user, config.password, &hosts, true); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/hosts", "error", err)
	}
	for _, stat := range hosts.Hosts[0].Stats {

		level.Debug(logger).Log("msg", "registering stat", "stat", stat.Name)

		hostMetrics[stat.Name] = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rhev_host_" + strings.ReplaceAll(stat.Name, ".", "_"),
			Help: stat.Description + " on host " + stat.Unit,
		}, []string{"cluster", "host"})
	}

	hostMetrics["up"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rhev_host_up",
		Help: "is rhev host active",
	}, []string{"cluster", "host"})

	hostMetrics["activeVMs"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rhev_host_activevms",
		Help: "number of active VMS on host",
	}, []string{"cluster", "host"})
}

func recordMetrics(config config, logger log.Logger) {

	for {
		level.Debug(logger).Log("msg", "metrics recording cycle")

		var vms VMs
		if err := call(config.endpoint, "/vms", config.user, config.password, &vms, false); err != nil {
			level.Error(logger).Log("call", config.endpoint + "/vms", "error", err)
		}
		var vmCount = make(map[string]float64)
		for _, vm := range vms.VMs {
			if vm.Host.Id != "" {
				vmCount[vm.Host.Id]++
			}
		}

		// record host states and stats (8s call)
		var hosts Hosts
		if err := call(config.endpoint, "/hosts", config.user, config.password, &hosts, true); err != nil {
			level.Error(logger).Log("call", config.endpoint + "/hosts", "error", err)
		}
		for _, host := range hosts.Hosts {
			level.Debug(logger).Log("msg", "recording host info", "host", host.Name, "state", host.Status)
			if host.Status == "up" {
				hostMetrics["up"].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(1)
			} else {
				hostMetrics["up"].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(0)
			}

			hostMetrics["activeVMs"].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(vmCount[host.Id])

			for _, stat := range host.Stats {
				hostMetrics[stat.Name].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(stat.Value)
			}
		}

		time.Sleep(time.Duration(config.interval) * time.Second)
	}
}
