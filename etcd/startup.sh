#!/bin/bash

token='epcache-registry-1'
cluster='infra1=http://localhost:2380,infra2=http://localhost:12380,infra3=http://localhost:22380'

# etcd1
nohup etcd --name infra1 --listen-client-urls http://localhost:2379 --advertise-client-urls http://localhost:2379 \
  --initial-advertise-peer-urls http://localhost:2380 --listen-peer-urls http://localhost:2380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new \
  --log-level warn >etcd1.log 2>&1 &

# etcd2
nohup etcd --name infra2 --listen-client-urls http://localhost:12379 --advertise-client-urls http://localhost:12379 \
  --initial-advertise-peer-urls http://localhost:12380 --listen-peer-urls http://localhost:12380 \
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new\
  --log-level warn >etcd2.log 2>&1 &

# etcd3
nohup etcd --name infra3 --listen-client-urls http://localhost:22379 --advertise-client-urls http://localhost:22379 \
   --initial-advertise-peer-urls http://localhost:22380 --listen-peer-urls http://localhost:22380\
  --initial-cluster-token $token --initial-cluster $cluster --initial-cluster-state new\
  --log-level warn >etcd3.log 2>&1 &

# run `./startup.sh` to start the etcd cluster in bg.
# This is just an example of three nodes, you may edit the file as you like to build your own etcd cluster.
