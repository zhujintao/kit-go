package bss

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alibabacloud-go/bssopenapi-20171214/v6/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
)

type Client = client.Client
type BillItem = client.DescribeInstanceBillResponseBodyDataItems
type InstanceItem = client.QueryAvailableInstancesResponseBodyDataInstanceList
type bss struct {
	*client.Client
}

func NewClient(accessKeyId, acessKeySecret string) *bss {
	endpoint := "business.aliyuncs.com"
	cli, err := client.NewClient(&openapiutil.Config{Endpoint: &endpoint, AccessKeyId: &accessKeyId, AccessKeySecret: &acessKeySecret})
	if err != nil {
		return nil
	}
	return &bss{cli}
}

// date in  YYYY-MM , day YYYY-MM-DD
func (cli *bss) GetBillDaily(day string) []*BillItem {

	granularity := "DAILY"
	req := client.DescribeInstanceBillRequest{Granularity: &granularity, BillingDate: &day}
	billingCycle := day[0:7]
	req.BillingCycle = &billingCycle

	/*
		_, err := time.Parse(time.DateOnly, *req.BillingDate)
		if err != nil {
			now := time.Now()
			granularity := now.Format(time.DateOnly)
			billingCycle := now.Format("2006-01")
			req.BillingDate = &granularity
			req.BillingCycle = &billingCycle

		}
	*/
	return cli.getBill(req)

}

// date in the YYYY-MM
func (cli *bss) GetBillMonthiy(date string) []*BillItem {

	granularity := "MONTHLY"
	req := client.DescribeInstanceBillRequest{Granularity: &granularity, BillingCycle: &date}

	/*
		_, err := time.Parse("2006-01", *req.BillingCycle)
		if err != nil {
			now := time.Now()
			billingCycle := now.Format("2006-01")
			req.BillingCycle = &billingCycle
		}
	*/

	return cli.getBill(req)

}

func (cli *bss) getBill(request client.DescribeInstanceBillRequest) []*BillItem {

	isBillingItem := true
	maxResults := int32(300)
	//granularity := "DAILY" //MONTHLY //DAILY

	var items []*client.DescribeInstanceBillResponseBodyDataItems
	var nextToken *string
	request.MaxResults = &maxResults
	request.IsBillingItem = &isBillingItem

a:

	request.NextToken = nextToken
	result, err := cli.DescribeInstanceBill(&request)
	if err != nil {
		return nil
	}
	if result.Body.Data == nil {
		fmt.Println(*result.Body.Message)
		return items
	}

	items = append(items, result.Body.Data.Items...)
	nextToken = result.Body.Data.NextToken
	if nextToken != nil {
		goto a
	}

	return items

}

func (cli *bss) GetSubscriptionStatus() []*InstanceItem {

	var items []*InstanceItem
	subscriptionType := "Subscription"
	pageSize := int32(300)
	result, err := cli.QueryAvailableInstances(&client.QueryAvailableInstancesRequest{PageSize: &pageSize, SubscriptionType: &subscriptionType})
	if result.Body.Data == nil {
		fmt.Println(err)
		return nil
	}

	items = append(items, result.Body.Data.InstanceList...)

	totalCount := *result.Body.Data.TotalCount
	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))
	for pageNumber := 2; pageNumber <= totalPages; pageNumber++ {
		pageNum := int32(pageNumber)
		result, err := cli.QueryAvailableInstances(&client.QueryAvailableInstancesRequest{PageSize: &pageSize, PageNum: &pageNum, SubscriptionType: &subscriptionType})
		if result.Body.Data == nil {
			fmt.Println(err)
			continue
		}
		items = append(items, result.Body.Data.InstanceList...)
	}

	return items

}

func (cli *bss) GetAvailableAmount() *float64 {

	result, err := cli.QueryAccountBalance()

	if err != nil || result.Body.Data == nil {
		fmt.Println(err)
		return nil
	}
	s := strings.ReplaceAll(*result.Body.Data.AvailableAmount, ",", "")
	a, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return &a

}
