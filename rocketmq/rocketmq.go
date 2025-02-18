package rocketmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/robertkrimen/otto"
	"github.com/zhujintao/kit-go/rocketmq/client"
	"github.com/zhujintao/kit-go/rocketmq/client/remote"
)

type nameserver struct {
	remote remote.RemotingClient
	ctx    context.Context
	addr   string
	broker map[string]*brokerData
}

type brokerData struct {
	*client.BrokerData
}

type customHeader struct {
	maps map[string]string
}

func (c *customHeader) Encode() map[string]string {
	return c.maps
}
func (c *customHeader) Add(k, v string) *customHeader {
	c.maps[k] = v
	return c
}

func NewCustomHeader(k, v string) *customHeader {
	return &customHeader{
		maps: map[string]string{k: v},
	}
}

func NewNameServer(nsAddr string) *nameserver {

	ns := &nameserver{
		ctx:    context.Background(),
		remote: remote.NewRemotingClient(nil),
		addr:   nsAddr,
		broker: make(map[string]*brokerData),
	}

	cmd := remote.NewRemotingCommand(int16(106), nil, nil)

	response, err := ns.remote.InvokeSync(ns.ctx, ns.addr, cmd)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	vm := otto.New()
	vm.Set("source", string(response.Body))
	value, err := vm.Run(`
	    var code = 'JSON.stringify(' + source + ')';
		eval(code);
	`)
	if err != nil {
		return nil
	}
	result, _ := value.ToString()

	var data struct {
		BrokerDataTable map[string]*client.BrokerData `json:"brokerAddrTable"`
		ClusterTable    map[string][]string           `json:"clusterAddrTable"`
	}
	json.Unmarshal([]byte(result), &data)

	for name, broker := range data.BrokerDataTable {

		ns.broker[name] = &brokerData{
			BrokerData: broker,
		}

	}

	return ns

}

func (ns *nameserver) GetClusterInfo() {
	//QueryBrokerClusterInfoFromServer
	cmd := remote.NewRemotingCommand(int16(106), nil, nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.addr, cmd)
	if err != nil {
		fmt.Println(err)
		return
	}

	vm := otto.New()
	vm.Set("source", string(response.Body))
	value, err := vm.Run(`
	    var code = 'JSON.stringify(' + source + ')';
		eval(code);
	`)
	if err != nil {
		return
	}
	result, _ := value.ToString()

	var data struct {
		BrokerDataTable map[string]*client.BrokerData `json:"brokerAddrTable"`
		ClusterTable    map[string][]string           `json:"clusterAddrTable"`
	}
	json.Unmarshal([]byte(result), &data)

	for name, broker := range data.BrokerDataTable {

		ns.broker[name] = &brokerData{
			BrokerData: broker,
		}

	}

}

func (ns *nameserver) GetBrokerRntimeInfo() {
	if len(ns.broker) == 0 {
		ns.GetClusterInfo()
	}
	fmt.Println(ns.broker)
	b := ns.broker["dc-sit-cd5ccb9d-f56nt"]
	addr := b.BrokerData.BrokerAddresses[0]
	cmd := remote.NewRemotingCommand(int16(28), nil, nil)
	response, err := ns.remote.InvokeSync(ns.ctx, addr, cmd)
	if err != nil {
		fmt.Println(err)
		return
	}
	var data map[string]map[string]string
	json.Unmarshal(response.Body, &data)
	table := data["table"]
	///bootTimestamp, _ := strconv.ParseInt(table["bootTimestamp"], 10, 64)
	//brokerVersion, _ := strconv.ParseInt(table["brokerVersion"], 10, 64)
	for k, v := range table {
		fmt.Println(k, v)
	}
}

func (ns *nameserver) ListTopic() []string {

	//ReqGetAllTopicListFromNameServer
	cmd := remote.NewRemotingCommand(int16(206), nil, nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.addr, cmd)
	if err != nil {
		return nil
	}
	var data map[string][]string
	json.Unmarshal(response.Body, &data)
	return data["topicList"]

}

func (ns *nameserver) GetTopicConsumeGroupList(topic string) []string {

	//ReqQueryTopicConsumeByWho
	cmd := remote.NewRemotingCommand(int16(300), NewCustomHeader("topic", topic), nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.brokerAddr(), cmd)
	fmt.Println(string(response.Body), err)
	var data map[string][]string
	json.Unmarshal(response.Body, &data)

	return data["groupList"]

}

//TPS 统计写消息量
//QPS 统计读消息量(消费)

func (ns *nameserver) GetTopicRouter(topic string) {
	// ReqGetRouteInfoByTopic

	cmd := remote.NewRemotingCommand(int16(105), NewCustomHeader("topic", topic), nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.addr, cmd)
	fmt.Println(string(response.Body), err)

}
func (ns *nameserver) brokerAddr() (addr string) {

	for _, brokers := range ns.broker {
		for _, addr = range brokers.BrokerAddresses {

		}
	}
	return
}

// statsKey is topic
//
// statsName: TOPIC_PUT_NUMS TOPIC_PUT_SIZE GROUP_GET_NUMS GROUP_GET_SIZE SNDBCK_PUT_NUMS
func (ns *nameserver) GetBrokerStats(statsName, statsKey string) {
	//ReqQueryBrokerStats
	cmd := remote.NewRemotingCommand(int16(315), NewCustomHeader("statsName", statsName).Add("statsKey", statsKey), nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.brokerAddr(), cmd)
	fmt.Println(string(response.Body), err)
}

func (ns *nameserver) GetConsumeStats(group, topic string) {
	cmd := remote.NewRemotingCommand(int16(208), NewCustomHeader("consumerGroup", group).Add("topic", topic), nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.brokerAddr(), cmd)
	fmt.Println(string(response.Body), err)
}

func (ns *nameserver) GetConsumerRunningInfo(group, clientId string) {
	//ReqGetConsumerRunningInfo
	cmd := remote.NewRemotingCommand(int16(307), NewCustomHeader("consumerGroup", group).Add("clientId", clientId).Add("jstackEnable", "false"), nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.brokerAddr(), cmd)
	fmt.Println(response.Remark, string(response.Body), err)
}
func (ns *nameserver) GetConsumerConnectionInfo(group string) {
	//ReqListConsumerConnection
	cmd := remote.NewRemotingCommand(int16(203), NewCustomHeader("consumerGroup", group), nil)
	response, err := ns.remote.InvokeSync(ns.ctx, ns.brokerAddr(), cmd)
	if err != nil {
		fmt.Println(err)
		return
	}
	var data map[string]interface{}
	json.Unmarshal(response.Body, &data)

	/*
			connectionSet <nil>
		consumeFromWhere <nil>
		consumeType <nil>
		messageModel <nil>
		subscriptionTable <nil>
	*/

}

// broker
// group (topic)
// nameserver
// topic
// consume
