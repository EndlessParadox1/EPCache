#!/bin/bash

token='epcache-registry-1'
cluster='infra1=http://localhost:2380,infra2=http://localhost:12380,infra3=http://localhost:22380'

# etcd1
nohup etcd --name infra1 --listen-client-urls http://localhost:2379 --advertise-client-urls http://localhost:2379 \
  --listen-peer-urls http://localhost:2380 --initial-advertise-peer-urls http://localhost:2380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new \
  --logger=warning --log-output=file:etcd1.log &

# etcd2
nohup etcd --name infra2 --listen-client-urls http://localhost:12379 --advertise-client-urls http://localhost:12379 \
  --listen-peer-urls http://localhost:12380 --initial-advertise-peer-urls http://localhost:12380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state \
  --logger=warning --log-output=file:etcd2.log &

# etcd3
nohup etcd --name infra3 --listen-client-urls http://localhost:22379 --advertise-client-urls http://localhost:22379 \
  --listen-peer-urls http://localhost:22380 --initial-advertise-peer-urls http://localhost:22380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state \
  --logger=warning --log-output=file:etcd3.log &

# run `./startup.sh` to start the etcd cluster in bg.
# This is just an example of three nodes, you may edit the file as you like to build your own etcd cluster.
