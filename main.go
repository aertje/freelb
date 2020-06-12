package main

import (
	"bytes"
	"context"
	"flag"
	"html/template"
	"log"
	"os"
	"os/exec"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

type TemplateData struct {
	Hosts []string
	Port  uint
}

func hostList(hostMap map[string]bool) []string {
	h := make([]string, len(hostMap))
	i := 0
	for k := range hostMap {
		h[i] = k
		i++
	}
	return h
}

func main() {
	kubeconfigPath := flag.String("kubeconfig", "/usr/local/etc/kubeconfig", "Absolute path to the kubeconfig file")
	templatePath := flag.String("template", "/usr/local/etc/nginx-template.conf", "Absolute path to the Nginx template")
	outputPath := flag.String("output", "/etc/nginx/sites-available/reverse-proxy.conf", "Absolute Nginx output path")

	selector := flag.String("selector", "monitor=proxy", "Label selector for pods")
	port := flag.Uint("port", 32657, "Nodeport on hosts to proxy")
	interval := flag.Uint("interval", 60, "Update interval")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		log.Panicln("Could not build config from flags:", err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln("Could not get clientset for config:", err.Error())
	}

	listOptions := metav1.ListOptions{
		LabelSelector: *selector,
	}

	template, err := template.ParseFiles(*templatePath)

	if err != nil {
		log.Panicln("Could not load Nginx template file:", err.Error())
	}

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	log.Println("Started")

	var lastHostMap map[string]bool

	for ; true; <-ticker.C {
		pods, err := clientset.CoreV1().Pods("").List(context.Background(), listOptions)
		if err != nil {
			log.Println("Could not retrieve pods:", err.Error())
			continue
		}

		// Use a map to avoid duplicates
		hostMap := make(map[string]bool)

		for _, pod := range pods.Items {
			host := pod.Status.HostIP
			if host != "" && pod.Status.Phase == corev1.PodRunning {
				hostMap[host] = true
			}
		}

		if len(hostMap) == 0 {
			log.Println("No relevant hosts identified")
			continue
		}

		if reflect.DeepEqual(hostMap, lastHostMap) {
			log.Println("Hosts did not change")
			continue
		}

		hosts := hostList(hostMap)
		log.Println("Found hosts:", hosts)

		fNginx, err := os.Create(*outputPath)
		if err != nil {
			log.Panicln("Could not create Nginx config file:", err.Error())
		}
		defer fNginx.Close()

		template.Execute(fNginx, TemplateData{Hosts: hosts, Port: *port})

		log.Println("Restarting Nginx...")
		cmd := exec.Command("systemctl", "restart", "nginx")

		// Retrieve stderr output in case of error
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			log.Panicln("Could not restart Nginx:", stderr.String())
		}

		log.Println("Nginx updated and restarted")
		lastHostMap = hostMap
	}
}
