package eks

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/zhujintao/kit-go/aws"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

type Client struct {
	*eks.Client
	ctx context.Context
	cfg *aws.Config
}

type kubconfig struct {
	ca          []byte
	token       string
	server      string
	clusterName string
}

func (k *kubconfig) WriteFile(id string, filename ...string) error {
	contextName := k.clusterName
	if id != "" {
		contextName = id + "@" + k.clusterName
	}
	c := api.Config{
		Clusters:       map[string]*api.Cluster{k.clusterName: {Server: k.server, CertificateAuthorityData: k.ca}},
		Contexts:       map[string]*api.Context{contextName: {AuthInfo: "admin", Cluster: k.clusterName}},
		AuthInfos:      map[string]*api.AuthInfo{"admin": {Token: k.token}},
		CurrentContext: contextName,
	}

	b, err := clientcmd.Write(c)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if len(filename) == 1 {

		return clientcmd.WriteToFile(c, filename[0])
	}
	fmt.Println(string(b))

	return nil

}

func NewClient(c *aws.Config) *Client {
	return &Client{Client: eks.NewFromConfig(*c), ctx: context.Background(), cfg: c}
}

func (c *Client) GetKubeConfig(clusterName string) *kubconfig {

	result, err := c.DescribeCluster(c.ctx, &eks.DescribeClusterInput{Name: &clusterName})
	if err != nil {
		return nil
	}

	cluster := result.Cluster
	g, _ := token.NewGenerator(true, false)
	token, err := g.GetWithSTS(*cluster.Name, sts.NewFromConfig(*c.cfg))
	if err != nil {
		fmt.Println(err)
		return nil
	}
	ca, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &kubconfig{
		ca:          ca,
		token:       token.Token,
		server:      *cluster.Endpoint,
		clusterName: *cluster.Name,
	}
}
