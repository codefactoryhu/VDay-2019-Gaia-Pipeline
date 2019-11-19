#### Run minikube on virtualbox, and set memory
```
minikube config set memory 12000
minikube start --vm-driver=virtualbox
```

#### Create docker network for gaia and vault
```
docker network create gaia-vault
```

#### Run hasicorp vault 
```
docker run --cap-add=IPC_LOCK -d \
    -e 'VAULT_DEV_ROOT_TOKEN_ID=root-token' \
    -e 'VAULT_ADDR=http://localhost:8200' \
    -e 'VAULT_TOKEN=root-token' \
    -p 8200:8200 --name=vault --net=gaia-vault vault:latest
```

#### Run Gaia Pipeline
```
docker run -d -p 8080:8080 --net=gaia-vault --name=gaia gaiapipeline/gaia:latest
```

#### Store Kube-Config into Vault
```
docker cp ~/.kube/config vault:/tmp/config
docker exec -it vault sh
vault kv put secret/kube-conf conf="$(cat /tmp/config | base64)"
```
#### SET variables on vault
```
Key: vault-token
Value: root-token

Key: vault-address
Value: http://vault:8200
```