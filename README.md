# FreeLB

A utility that polls `kubectl` at a regular basis in order to resolve pods that are hosting an app, and updates the local nginx configuration to proxy those pods.

This allows you to run a poor man's Load Balancer in front of your kube cluster by attaching a static IP to a normal VM, and exposing the cluster via a NodePort.

Currently configured to be used with a Google Kubernetes Engine cluster, but can easily be modified for other providers.
