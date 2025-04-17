#!/bin/bash

set -x
set -o errexit
set -o pipefail
set -o nounset

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../..; pwd)

IMAGE_NAME=${IMAGE_NAME:-"ghcr.io/infrastructure-io/topohub-tools:latest"}
IMAGE_VERSION=${IMAGE_VERSION:-"latest"}
CLUSTER_NAME=${CLUSTER_NAME:-"topohub"}
PYROSCOPE_LOCAL_PORT=${PYROSCOPE_LOCAL_PORT:-""}
PYROSCOPE_CONTAINER_NAME=${PYROSCOPE_CONTAINER_NAME:-"pyroscope"}
IMAGE_PYROSCOPE_NAME=${IMAGE_PYROSCOPE_NAME:-"pyroscope/pyroscope:latest"}

#====================================

echo "Deploying application using Helm chart..."

helm uninstall topohub -n topohub --wait &>/dev/null || true

echo "run topo on worker nodes"
kubectl label node ${CLUSTER_NAME}-worker topohub=true

cat <<EOF >/tmp/topo.yaml
replicaCount: 1
logLevel: "debug"
image:
  tag: "${IMAGE_VERSION}"

defaultConfig:
  redfish:
    https: false
    port: 8000
    username: ""
    password: ""
  dhcpServer:
    interface: "eth1"

storage:
  type: "hostPath"

nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: topohub
        operator: In
        values:
        - "true"

fileBrowser:
  enabled: true
  port: 8080
EOF

IMAGE_LIST=$( helm template topohub ${PROJECT_ROOT_PATH}/chart -f /tmp/topo.yaml  | grep "image:" | awk '{print $2}' | tr -d '"' )
[ -n "${PYROSCOPE_LOCAL_PORT}" ] && IMAGE_LIST+=" ${IMAGE_PYROSCOPE_NAME} "
for IMAGE in $IMAGE_LIST; do
    echo "loading $IMAGE"
    docker inspect $IMAGE &>/dev/null || docker pull $IMAGE  
    kind load docker-image $IMAGE --name ${CLUSTER_NAME}
done

HELM_OPTION=" --wait -f /tmp/topo.yaml"

if [ -n "${PYROSCOPE_LOCAL_PORT}" ]; then
    docker stop ${PYROSCOPE_CONTAINER_NAME} &>/dev/null || true
    docker rm ${PYROSCOPE_CONTAINER_NAME} &>/dev/null || true
    ServerAddress=$(docker network inspect kind -f '{{(index .IPAM.Config 0).Gateway}}')
    echo "setup pyroscope on ${ServerAddress}:${PYROSCOPE_LOCAL_PORT}"
    docker run -d --name ${PYROSCOPE_CONTAINER_NAME} -p ${PYROSCOPE_LOCAL_PORT}:4040 ${IMAGE_PYROSCOPE_NAME} server
    echo "set env to topohub"
    HELM_OPTION+=" --set extraArgs[0]=--pyroscope-address=http://${ServerAddress}:${PYROSCOPE_LOCAL_PORT}"
    HELM_OPTION+=" --set extraArgs[1]=--pyroscope-tag=topohub"
    echo "finish setuping pyroscope"
fi

helm install topohub ${PROJECT_ROOT_PATH}/chart \
    --namespace topohub \
    --create-namespace \
    --debug \
    ${HELM_OPTION}
