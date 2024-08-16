#!/bin/bash

num=3
names=(infra1 infra2 infra3)
urls=(http://localhost:2379 http://localhost:12379 http://localhost:22379)
peerUrls=(http://localhost:2380 http://localhost:12380 http://localhost:22380)
token=epcache-registry-1
cluster='infra1=http://localhost:2380,infra2=http://localhost:12380,infra3=http://localhost:22380'
logs=(etcd1.log etcd2.log etcd3.log)

start() {
  nohup etcd --name "$1" --listen-client-urls "$2" --advertise-client-urls "$2" \
    --initial-advertise-peer-urls "$3" --listen-peer-urls "$3" \
    --initial-cluster-token "$token" --initial-cluster "$cluster" --initial-cluster-state new \
    --log-level warn >"$4" 2>&1 &
}

for (( i = 0; i < num; i++ )); do
    start "${names[$i]}" "${urls[$i]}" "${peerUrls[$i]}" "${logs[$i]}"
done

# run `./startup.sh` to start the etcd cluster in bg.
# This is just an example of three nodes, you may edit the file as you like to build your own etcd cluster.
