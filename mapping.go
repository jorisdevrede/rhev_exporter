package main

type Clusters struct {
        Clusters []Cluster `xml:"cluster"`
}

type Cluster struct {
        Id string `xml:"id,attr"`
        Name string `xml:"name,omitempty"`
}

type Hosts struct {
        Hosts []Host `xml:"host"`
}

type Host struct {
        Id string `xml:"id,attr"`
        Name string `xml:"name,omitempty"`
        Status Status `xml:"status,omitempty"`
        Cluster Cluster `xml:"cluster,omitempty"`
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
        Host Host `xml:"host"`
        Values Values `xml:"values"`
}

type Values struct {
        Value Value `xml:"value"`
}

type Value struct {
        Datum float64 `xml:"datum"`
}
