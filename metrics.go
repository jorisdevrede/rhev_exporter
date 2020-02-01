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
)

var (
        hostMetrics    = make(map[string]*prometheus.GaugeVec)
)

// calls RHEV REST API and translates xml response to a struct
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

// initializes metrics for export
func initMetrics(config config, logger log.Logger) {

	level.Info(logger).Log("msg", "initalizing metrics")

	// retrieve cluster Names and Ids
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

	// retrieve stats and register them as metrics for export
	var stats Stats
	if err := call(config.endpoint, "/hosts/" + hosts.Hosts[0].Id + "/statistics", config.user, config.password, &stats); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/hosts/" + hosts.Hosts[0].Id + "/statistics" ,"error", err)
	}
	for _, stat := range stats.Stats {

		level.Debug(logger).Log("msg", "registering stat", "stat", stat.Name)

		hostMetrics[stat.Name] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_host_" + strings.ReplaceAll(stat.Name, ".", "_"),
                        Help: stat.Description + " on host " + stat.Unit,
                },[]string{"cluster", "host"})
	}

	hostMetrics["up"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_host_up",
                        Help: "is rhev host active",
                },[]string{"cluster", "host"})

	hostMetrics["activeVMs"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rhev_host_activevms",
			Help: "number of active VMS on host",
		},[]string{"cluster", "host"})
}

func recordMetrics(config config, logger log.Logger) {

	for {
		level.Debug(logger).Log("msg", "metrics recording cycle")

		// record host states
	        var hosts Hosts
		if err := call(config.endpoint, "/hosts", config.user, config.password, &hosts); err != nil {
			level.Error(logger).Log("call", config.endpoint + "/hosts","error", err)
		}
		for _, host := range hosts.Hosts {
			level.Debug(logger).Log("msg", "recording host info", "host", host.Name, "state", host.Status.State)
			if host.Status.State == "up" {
				hostMetrics["up"].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(1)
			} else {
				hostMetrics["up"].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(0)
			}

			// record host statistics
			var stats Stats
			if err := call(config.endpoint, "/hosts/" + host.Id + "/statistics", config.user, config.password, &stats); err != nil {
				level.Error(logger).Log("call", config.endpoint + "/hosts/" + host.Id + "/statistics" ,"error", err)
			}
			for _, stat := range stats.Stats {
				hostMetrics[stat.Name].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name}).Set(stat.Values.Value.Datum)
			}

			// record vms per host
			var vms VMs
			if err := call(config.endpoint, "/vms?search=host%3D" + host.Name, config.user, config.password, &vms); err != nil {
				level.Error(logger).Log("call", config.endpoint + "/vms?search=host%3D" + host.Name, "error", err)
			}
			activeVMs := float64(len(vms.VMs))
			hostMetrics["activeVMs"].With(prometheus.Labels{"cluster": clusterList[host.Cluster.Id], "host": host.Name }).Set(activeVMs)
		}

		time.Sleep(time.Duration(config.interval) * time.Second)
	}
}
