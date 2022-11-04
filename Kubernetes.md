# kubelet

- Definition : kubelet là thành phần chạy trên tất cả các **machine** trong cluster của mình và làm những việc như khởi động các POD và container

- Nói cách khác thì nó là 1 tiến trình chạy trên mỗi worker node và có thể tạo xóa hoặc nâng cấp các pod và docker container dựa theo yêu cầu mà nó nhận được từ API server

![image](https://itknowledgeexchange.techtarget.com/coffee-talk/files/2021/03/kubelet-1024x933.png)

> Dễ thấy trong hình thì **mỗi cụm worker node** đều có **1 instance chạy kubelet**
> Khi nó **nhận được request từ controller node** thì **nó sẽ tạo** các pod và các container
> Cơ chế sẽ là **controller node sẽ thông báo** cho từng instance **kubelet** trên mối worker node để **tương tác** với các **worker node container runtime** để tạo ra container
> Sau đó Kubelet làm việc với controller node để điều phối các container vào các pod mà nó được chỉ định

- **Phân biệt kubectl và kubelet:**
  - **Kubelet** được sử dụng để tương tác làm việc với worker-node. Hay nói dễ hiểu **Kubelet là cơ chế của K8s để tạo ra các container trong một workernode**
  - **Kubectl** là một CLI tool để các developer có thể tương tác với cụm cluster ở mọi nơi bất kỳ nơi nào cũng được. (kiểu 1 dạng kết nối đến cụm cluster tương tác với cụm cluster thông qua CLI nó dùng HTTP POST)

---

# kubeadm

- Definition: Kubeadm là một công cụ giúp tự động hóa quá trình cài đặt và triển khai kubernetes trên môi trường Linux, do chính kubernetes hỗ trợ
- a

---

# kubectl

- Definition : là một CLI tool để các developer có thể tương tác với cụm cluster ở mọi nơi bất kỳ nơi nào cũng được. Cơ chế là Kubectl cung cấp CLI interface để user có thể tương tác được với k8s API cluster phục vụ các tác vụ như thêm sửa xóa các resource của k8s

---

# kubebuilder

- Definition : là 1 tool phục vụ cho việc automation generate ra các resource trong k8s
Kubebuilder - SDK for building Kubernetes APIs using CRDs
Nó là 1 tool khởi tạo ra các resource dưới dạng code go. ví dụ mình tạo ra các API,..

- Idea của nó tương tự như framework nhưng điểm khác biệt là nó là 1 framework phục vụ cho việc xây dựng API của K8s bằng các sử dụng Custom resource definitions(CRDs).

```console
kubebuilder create api --group webapp --version v1 --kind Guestbook
```


## [Architect](https://kube.academy/courses/the-kubernetes-machine)
