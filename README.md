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