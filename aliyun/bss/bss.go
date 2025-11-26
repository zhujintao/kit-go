package bss

import (
	"errors"

	"github.com/alibabacloud-go/bssopenapi-20171214/v6/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
)

type Client = *client.Client
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
func (cli *bss) GetBillDaily(day string) ([]*client.DescribeInstanceBillResponseBodyDataItems, error) {

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
func (cli *bss) GetBillMonthiy(date string) ([]*client.DescribeInstanceBillResponseBodyDataItems, error) {

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

func (cli *bss) getBill(request client.DescribeInstanceBillRequest) ([]*client.DescribeInstanceBillResponseBodyDataItems, error) {

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
		return nil, err
	}
	if result.Body.Data == nil {
		return items, errors.New(*result.Body.Message)
	}

	items = append(items, result.Body.Data.Items...)
	nextToken = result.Body.Data.NextToken
	if nextToken != nil {
		goto a
	}

	return items, nil

}
