#!/bin/bash
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./ns1.infra.local/0 "OS=linux;ENV=dev;ROLE=platform;SRV=nameservers;VARS=NS=first,START=true"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./etcd-1.infra.local/0 "OS=linux;ENV=dev;ROLE=platform;SRV=dbs;VARS=ETCD_VARS=first,START=true"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./etcd-2.infra.local/0 "OS=linux;ENV=dev;ROLE=platform;SRV=dbs;VARS=ETCD_VARS=first,START=true"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./etcd-3.infra.local/0 "OS=linux;ENV=dev;ROLE=platform;SRV=dbs;VARS=ETCD_VARS=first,START=true"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./bastion.infra.local/0 "OS=linux;ENV=dev;ROLE=platform;SRV=tools;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./master-1.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=controlplane;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./master-2.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=controlplane;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./master-3.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=controlplane;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./ingress-1.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=ingresses;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./ingress-2.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=ingresses;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./ingress-3.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=ingresses;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./worker-1.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=workers;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./worker-2.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=workers;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./observability-1.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=observability;VARS=key1=value1,key2=value2"
docker compose -f docker/docker-compose.yml exec -it etcd-1 \
    etcdctl put ANSIBLE_INVENTORY/infra.local./logging-1.infra.local/0 "OS=linux;ENV=dev;ROLE=k8s;SRV=logs;VARS=key1=value1,key2=value2"