echo"   "
echo"=================================="
echo"        install package dependencies git make docker python"
echo"=================================="
apt update && apt upgrade -y 
apt install -y git make curl wget docker.io python3.9 python3-pip python3-venv
echo"   "
echo"=================================="
echo"        install done"
echo"=================================="
echo"   "
echo"=================================="
echo"        install go-1.19"
echo"=================================="
wget https://go.dev/dl/go1.19.linux-amd64.tar.gz 
tar -xvf go1.19.linux-amd64.tar.gz 
cp /go/bin/go /usr/local/bin/go && mv go /usr/local  
export GOROOT=/usr/local/go 
rm -r go1.19.linux-amd64.tar.gz
echo"   "
echo"=================================="
echo "      Check go version"
echo"=================================="
go version
echo"   "
echo"=================================="
echo"       install kubectl kustomize gg-cloud go-cli telegram"
echo"=================================="
snap install kubectl --classic
snap install kustomize --classic
snap install google-cloud-cli --classic
snap install golangci-lint --classic
snap install telegram-desktop
echo"   "
echo"=================================="
echo"       install done"
echo"=================================="
echo"   "
echo"=================================="
echo"       install kind clusterctl kubebuilder"
echo"=================================="
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.17.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
cd cluster-api && git checkout release-0.4 && sleep 3 && make clusterctl && cp ./bin/clusterctl /usr/local/bin/clusterctl
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
echo"   "
echo"=================================="
echo"      install done"
echo"=================================="
