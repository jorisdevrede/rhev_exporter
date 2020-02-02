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
        Status string `xml:"status>state,omitempty"`
        Cluster Cluster `xml:"cluster,omitempty"`
	Stats []Stat `xml:"statistics>statistic,omitempty"`
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
	Value float64 `xml:"values>value>datum"`
}
