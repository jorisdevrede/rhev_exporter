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

type ch struct { cluster, host string}

var (
	clusterList = make(map[string]string)
	hostList    = make(map[string]ch)
)

var (
        clusterMetrics = make(map[string]*prometheus.GaugeVec)
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

	// retrieve host Names and Ids, combined with cluster Names
	var hosts Hosts
	if err := call(config.endpoint, "/hosts", config.user, config.password, &hosts); err != nil {
		level.Error(logger).Log("call", config.endpoint + "/hosts","error", err)
	}
	for _, host := range hosts.Hosts {
		hostList[host.Id] = ch{clusterList[host.Cluster.Id], host.Name}
	}

	// retrieve stats and register them as metrics for export
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

		hostMetrics[stat.Name] = promauto.NewGaugeVec(prometheus.GaugeOpts{
                        Name: "rhev_host_" + strings.ReplaceAll(stat.Name, ".", "_"),
                        Help: stat.Description + " on host " + stat.Unit,
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

		type metrics struct {cluster, metric string}
                sum := make(map[metrics]float64)

		// record host states
	        var hosts Hosts
		if err := call(config.endpoint, "/hosts", config.user, config.password, &hosts); err != nil {
			level.Error(logger).Log("call", config.endpoint + "/hosts","error", err)
		}
		for _, host := range hosts.Hosts {
			level.Debug(logger).Log("msg", "recording host info", "host", host.Name, "state", host.Status.State)
			if host.Status.State == "up" {
				sum[metrics{clusterList[host.Cluster.Id], "activeHosts"}] = sum[metrics{clusterList[host.Cluster.Id], "activeHosts"}] + 1
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
				// TODO - differentiatie between unit types
				sum[metrics{hostList[stat.Host.Id].cluster, stat.Name}] = sum[metrics{hostList[stat.Host.Id].cluster, stat.Name}] + stat.Values.Value.Datum
				hostMetrics[stat.Name].With(prometheus.Labels{"cluster": hostList[stat.Host.Id].cluster, "host": hostList[stat.Host.Id].host}).Set(stat.Values.Value.Datum)
			}
		}

               for key, value := range sum {
                        // record cluster metrics
                        clusterMetrics[key.metric].With(prometheus.Labels{"cluster": key.cluster}).Set(value)
                }

		time.Sleep(time.Duration(config.interval) * time.Second)
	}
}
