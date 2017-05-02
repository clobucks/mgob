package api

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stefanprodan/mgob/config"
	"net/http"
)

type HttpServer struct {
	Config *config.AppConfig
}

func (s *HttpServer) Start(version string) {
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/version", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, version)
	})

	http.Handle("/", http.FileServer(http.Dir(s.Config.StoragePath)))

	logrus.Error(http.ListenAndServe(fmt.Sprintf(":%v", s.Config.Port), http.DefaultServeMux))
}