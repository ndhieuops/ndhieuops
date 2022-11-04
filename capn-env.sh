apt update && apt upgrade -y 
apt install -y git make 
wget  https://go.dev/dl/go1.19.linux-amd64.tar.gz 
tar -xvf go1.19.linux-amd64.tar.gz 
cp /go/bin/go /usr/local/bin/go && mv go /usr/local  
export GOROOT=/usr/local/go 
rm -r go go1.19.linux-amd64.tar.gz
go version
snap install kubectl --classic
snap install kustomize --classic
snap install google-cloud-cli --classic
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.17.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
git clone https://github.com/kubernetes-sigs/cluster-api-provider-nested.git
git clone https://github.com/kubernetes-sigs/cluster-api.git
cd cluster-api && git checkout release-0.4 && make clusterctl && chmod +x clusterctl && cp clusterctl /urs/local/bin/clusterctl
wget https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
