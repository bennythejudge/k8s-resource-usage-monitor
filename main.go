// main.go - safe version without OpenAPI handler
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type podResourceInfo struct {
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	Requests   map[string]string `json:"requests,omitempty"`
	Limits     map[string]string `json:"limits,omitempty"`
	Usage      map[string]string `json:"usage,omitempty"`
	MetricsErr string            `json:"metricsError,omitempty"`
}

func main() {
	port := flag.String("port", "8443", "HTTPS port")
	certFile := flag.String("tls-cert", "/tls/tls.crt", "TLS certificate file")
	keyFile := flag.String("tls-key", "/tls/tls.key", "TLS key file")
	flag.Parse()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("In-cluster config error: %v", err)
	}
	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}
	metricsClient, err := metricsv.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create metrics client: %v", err)
	}

	// Health endpoint
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// OpenAPI endpoint: return 404 to avoid apiserver crash
	http.HandleFunc("/openapi/v2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	// Discovery endpoint (mandatory)
	http.HandleFunc("/apis/example.com/v1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resourceList := &metav1.APIResourceList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "APIResourceList",
			},
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pod-resources",
					Namespaced: true,
					Kind:       "PodResourceInfo",
					Verbs:      []string{"get", "list"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resourceList)
	})

	// Custom API handler
	http.HandleFunc("/apis/example.com/v1/namespaces/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/apis/example.com/v1/namespaces/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "pod-resources" {
			http.NotFound(w, r)
			return
		}
		namespace := parts[0]
		podName := r.URL.Query().Get("pod")
		labelSelector := r.URL.Query().Get("labelSelector")

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		var fieldSelector string
		if podName != "" {
			fieldSelector = fmt.Sprintf("metadata.name=%s", podName)
		}

		podList, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}
		if len(podList.Items) == 0 {
			http.Error(w, "no pods found", http.StatusNotFound)
			return
		}

		metricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("Warning: failed to get metrics: %v", err)
		}
		metricsMap := make(map[string]metricsv1beta1.PodMetrics)
		for _, m := range metricsList.Items {
			metricsMap[m.Name] = m
		}

		result := []podResourceInfo{}
		for _, pod := range podList.Items {
			info := podResourceInfo{Namespace: pod.Namespace, Name: pod.Name}

			reqs := make(map[string]string)
			lims := make(map[string]string)
			for _, container := range pod.Spec.Containers {
				if cpu := container.Resources.Requests.Cpu(); cpu != nil {
					reqs["cpu"] = cpu.String()
				}
				if mem := container.Resources.Requests.Memory(); mem != nil {
					reqs["memory"] = mem.String()
				}
				if cpu := container.Resources.Limits.Cpu(); cpu != nil {
					lims["cpu"] = cpu.String()
				}
				if mem := container.Resources.Limits.Memory(); mem != nil {
					lims["memory"] = mem.String()
				}
			}
			info.Requests = reqs
			info.Limits = lims

			if metric, ok := metricsMap[pod.Name]; ok {
				usage := make(map[string]string)
				var totalCPU, totalMem int64
				for _, c := range metric.Containers {
					if cpu := c.Usage.Cpu(); cpu != nil {
						totalCPU += cpu.MilliValue()
					}
					if mem := c.Usage.Memory(); mem != nil {
						totalMem += mem.Value()
					}
				}
				usage["cpu"] = fmt.Sprintf("%dm", totalCPU)
				usage["memory"] = fmt.Sprintf("%dMi", totalMem/(1024*1024))
				info.Usage = usage
			} else {
				info.MetricsErr = "metrics not available (pod may be too new or metrics-server not ready)"
			}
			result = append(result, info)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Printf("Starting server on :%s", *port)

	server := &http.Server{
		Addr: ":" + *port,
		TLSConfig: &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				log.Printf("TLS handshake: client %v SNI=%s", info.Conn.RemoteAddr(), info.ServerName)
				cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
				if err != nil {
					log.Printf("ERROR loading certificate: %v", err)
					return nil, err
				}
				return &cert, nil
			},
		},
	}
	log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
}
