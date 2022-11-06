# Tổng kết họp ngày 3/11/2022

## Agenda

- [Tổng kết họp ngày 3/11/2022](#tổng-kết-họp-ngày-3112022)
  - [Agenda](#agenda)
    - [I. Về kiến trúc hiện tại](#i-về-kiến-trúc-hiện-tại)
      - [1. Mô hình tổng quan](#1-mô-hình-tổng-quan)
      - [2. Mô hình kiến trúc cũ](#2-mô-hình-kiến-trúc-cũ)
      - [3. Flow triển khai](#3-flow-triển-khai)
      - [4. Các thành phần](#4-các-thành-phần)
      - [5. Ưu điểm](#5-ưu-điểm)
      - [6. Nhược điểm](#6-nhược-điểm)
  - [II. Về kiến trúc mới](#ii-về-kiến-trúc-mới)
  - [III. Câu hỏi tìm hiểu neu ra CRD của kiến trúc hiện tại CAPI, KubeadmBootstrap, CAPO, KubeadmControlplane](#iii-câu-hỏi-tìm-hiểu-neu-ra-crd-của-kiến-trúc-hiện-tại-capi-kubeadmbootstrap-capo-kubeadmcontrolplane)

### I. Về kiến trúc hiện tại

#### 1. Mô hình tổng quan

#### 2. Mô hình kiến trúc cũ

![image](https://github.com/ndhieuops/ndhieuops/blob/note/report.png)

#### 3. Flow triển khai

- Từ cluster API mình sẽ define control plane mà nó sẽ dùng là cái nào và cả infrastructure mà nó dùng là infra bên nào
  - B1: từ thằng infrastructure đó nó sẽ tự động tạo 1 LoadBalancer cho APi server
  - B2: từ thằng control plane thì nó sẽ tạo ra 1 CRD Machine ở CRD này mình sẽ define ra infrastructurespec và infrastructurespec temmplate

#### 4. Các thành phần

- **CLuster API Provider** : Hiện tại mình đang dùng core của [Cluster API]
    > **Nhiệm vụ :** nó sẽ tạo ra các
    >
    > - Thành phần **CRD** :
    >   - **Cluster**
    >     - Trong spec của nó có :
    >       - **clusterconfiguration** cùng với **initConfiguration** là những cấu hình cần thiết cho init command
    >       - **JoinConfiguration** là những cấu hình kubeadm configuration cho join command
    >   - **Machine**
    >     - Trong spec của nó có :
    >       - **Template** : nó sẽ định nghĩa ra cấu trúc của template cho kubeadm config như init configuration, join configuration và cluster configuration
    >   - **Machine Deployment**
    >     - Trong spec của nó có :
    >       - **Template** : nó sẽ định nghĩa ra cấu trúc của template cho kubeadm config như init configuration, join configuration và cluster configuration
    >   - **Machine Healthcheck**
    >     - Trong spec của nó có :
    >       - **Template** : nó sẽ định nghĩa ra cấu trúc của template cho kubeadm config như init configuration, join configuration và cluster configuration

- **Cluster API Provider BootStrap** : Hiện tại thì mình cũng đang dùng **Kubeadm BootStrap** của [Cluster API]
    > **Nhiệm vụ :** nó sẽ tạo ra các data config như cluster configuration hay init configuration hoặc joinconfiguration. Tức là nó sẽ tạo ra các file cấu hình hoặc các template init để khi 1 VM nó boot lên thì sẽ apply các template vào các workernode đó.
    >
    > - Thành phần **CRD** : 2 thành phần chính
    >   - **kubeadm config**
    >     - Trong spec của nó có :
    >       - **clusterconfiguration** cùng với **initConfiguration** là những cấu hình cần thiết cho init command
    >       - **JoinConfiguration** là những cấu hình kubeadm configuration cho join command
    >   - **kubeadm config templates**
    >     - Trong spec của nó có :
    >       - **Template** : nó sẽ định nghĩa ra cấu trúc của template cho kubeadm config như init configuration, join configuration và cluster configuration
    >

- **Cluster API Provider Controlplane** : Đối với Controlplane thì mình đang dùng **Kubeadm Controlplane** của [Cluster API]
    > **Nhiệm vụ :** nó sẽ chịu trách nhiệm quản lý các cấu hình để boot lên 1 cụm control plane
    >
    > - Thành phần **CRD** : 2 thành phần chính
    > **kubeadmcontrolplanes**
    >   - Trong spec của nó có :
    >     - **infrastructure Template** : 
    >     - **kubeadm config spec** : được sử  dụng cho việc khởi tạo và join các **Machine** vào **controlplane**
    > **kubeadmcontrolplanes template**
    >   - Trong spec của nó có :
    >   - **infrastructure** : a

- **Infrastructure Provider** : Hiện tại thì mình đang dùng **Cluster API Provider OpenStack** ([CAPO])
    > **Nhiệm vụ :** nó sẽ chịu trách nhiệm tạo ra các resource tương ứng dưới lớp hạ tầng như các VM, LoadBalancer...
    >
    > - Thành phần **CRD** : 4 thành phần chính
    >   - **Openstack cluster infrastructure**
    >     - Trong spec của nó có :
    >       - **infrastructure Template** : Cung cấp template cho ha tang tuyỳ theo nhu cau tai nguyeên deẻ noó reêrenece voiơ thang ina structuer provider ( hay noi cach khac la de tao deuowcj cac cutom reource  tren tahngf inifra provider thiìcaâầpha co template)
    >       - **kubeadm config spec** : được sử  dụng cho việc khởi tạo và join các **Machine** vào **controlplane**
    >   - **Openstack cluster infrastructure template**
    >     - Trong spec của nó có :
    >       - **infrastructure** : cung cấp các template tương ứng với  các spec mà mình đề ra ở trên
    >   - **Openstack Machine infrastructure**
    >     - Trong spec của nó có :
    >       - **infrastructure Template** :
    >       - **kubeadm config spec** :
    >   - **Openstack Machine infrastructure template**
    >     - Trong spec của nó có :
    >       - **infrastructure** : cung cấp các template tương ứng với  các spec mà mình đề ra ở trên

#### 5. Ưu điểm

#### 6. Nhược điểm

- Ở mô hình cũ thì mình sử dụng thằng control plane của thằng kubeadm nó sẽ boot các control plane đó dưới dạng các VM dẫn đến việc boot nên khá chậm ~~ 9 phút( vi de boot duoc thi dau tien no se boot matrer node truoc sau do se den ưoửke node va tuonng tu lan luot cac mater node va worker node con lai ) --> tim giai phap de giam thoi gian boot
- Ngoài ra thì việc để hết các master node cùng ở với các worker node dẫn đến việc nếu người dùng có the tác động den caác master node.

## II. Về kiến trúc mới

- Ly do :De giai quyet nhung van de con ton dong  mo hinh kien truc cu. Thi doi voi thang CAPC(Cluster Api Controlplane) minh co the ap dung giai phap cua thang nested -> no se khoi tao cac control plane thay vi duoi dang kubeadm la cac Vitual Machine thi se la cac Pod. Va hon nua thi de quan ly cac pod do thi no se duoc trien khai tap trung tren cum cluster cua minh --> giai quyet van de distributed maáeed node va worker node.
- Khi trien khai duoi dang cac pod thi se tang thoi gian boot len tu 9 phut --> 4 5p ( theo ly thuyet)



## III. Câu hỏi tìm hiểu neu ra CRD của kiến trúc hiện tại CAPI, KubeadmBootstrap, CAPO, KubeadmControlplane

- Bộ CRD của CAPI
- Bộ CRD của CAPO
- Bộ CRD của KubeadmBootstrap
- Bộ CRD của KubeadmControlplane

--- 

Phan tich

Với CAPI thì đầu tiên khi khởi tạo nó sẽ tạo ra 1 event resource Machine Health check thì nó sẽ đảm bảo cho cái gì ?

Sau đó lại 1 event khác là Machine Set để làm gì ? (nó là boootstrap để set cả infrastructure reference)
(2 cái khác nhau) cái thứ nhất là set template cái thứ 2 là fill vào template các spec tương ứng

Tiếp nó lại fill lại vào machine health check

- Tức là machine set là arg để fill vào machine health check ?

Tiếp theo nó tạo ra clsuter có spec là control olane endpont sau đó nó lại tạo tiếp 1 clsuter nữa với status boootstrap false

Thêm 1 lần nữa nó tạo ra manchine health check để add thêm các spec control plane endpoint

Step tiếp theo nó tạo ra controller với reconciler là machine heatlh check

Sau đó nó tạo machineset và cluster
--> nó tạo ra machine deployment  và lần lượt tạo lại machine và cluster
Nó lại recociler các machinedeployment tương ứng --> khởi tạo các worker node.

Nó Adding watcher on external object để phục vụ cho việc

Note : Mình sử dụng thằng cluster API trước để init khởi tạo các resource mình chỉ định như là --core là gì ...

sau khi khởi tạo xong thì mình sẽ apply các template để tạ ra các resource tương ứng
hay nói cách khác init tạo ra các arg còn tempalte thì fill in các arg vào đó ?


---
[Cluster API]:<https://github.com/kubernetes-sigs/cluster-api>
[CAPO]:<https://github.com/kubernetes-sigs/cluster-api-provider-openstack.git>
