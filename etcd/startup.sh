#!/bin/bash

token='epcache-registry-1'
cluster='infra1=http://127.0.0.1:2380,infra2=http://127.0.0.1:12380,infra3=http://127.0.0.1:22380'

# etcd1
nohup etcd --name infra1 --listen-client-urls http://127.0.0.1:2379 --advertise-client-urls http://127.0.0.1:2379 \
  --listen-peer-urls http://127.0.0.1:2380 --initial-advertise-peer-urls http://127.0.0.1:2380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new >etcd1.log 2>&1 &

# etcd2
nohup etcd --name infra2 --listen-client-urls http://127.0.0.1:12379 --advertise-client-urls http://127.0.0.1:12379 \
  --listen-peer-urls http://127.0.0.1:12380 --initial-advertise-peer-urls http://127.0.0.1:12380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new >etcd2.log 2>&1 &

# etcd3
nohup etcd --name infra3 --listen-client-urls http://127.0.0.1:22379 --advertise-client-urls http://127.0.0.1:22379 \
  --listen-peer-urls http://127.0.0.1:22380 --initial-advertise-peer-urls http://127.0.0.1:22380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new >etcd3.log 2>&1 &

# run `./startup.sh` to start the etcd cluster in bg.
# This is just an example of three nodes, you may edit the file as you like to build your own etcd cluster.
