package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	sqbvault "git.wosai-inc.com/middleware/sqb-vault-sdk-go"
	"git.wosai-inc.com/middleware/sqb-vault-sdk-go/instrumentation/sls"
	"github.com/stretchr/testify/assert"
	pb "github.com/wosai/havok/pkg/genproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	testDecoder struct{}
)

func (d *testDecoder) Name() string {
	return "test-decoder"
}

func (d *testDecoder) Decode(b []byte) (*pb.LogRecord, error) {
	var data = map[string]interface{}{}
	err := json.Unmarshal(b, &data)
	if err != nil {
		return nil, err
	}

	sec, err := strconv.ParseInt(data["__time__"].(string), 10, 64)
	if err != nil {
		return nil, err
	}
	return &pb.LogRecord{
		Url:     "http://github.com",
		Method:  "get",
		OccurAt: timestamppb.New(time.Unix(sec, 0)),
	}, nil
}

func TestSLSFetcher_Apply(t *testing.T) {
	_, err := NewSLSClient(&SLSClientOption{
		Concurrency: 1,
		Endpoint:    "xxx.aliyuncs.com",
	}, time.Now(), time.Now())
	assert.Nil(t, err)
}

func TestSLSFetcher_WithDecoder(t *testing.T) {
	f := &SLSClient{}
	f.WithDecoder(&testDecoder{})
}

func TestNewSLSClient(t *testing.T) {
	defer func() {
		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()
		// 清理SQB Vault资源
		// 须确保SQB Vault的生命周期与你的应用程序生命周期相同，因此我们建议将此处的代码放置与main中执行
		if err := sqbvault.Finalize(ctx); err != nil {
			fmt.Printf("[WARN] finalize sqb vault error: %v\n", err)
		}
	}()

	if err := sqbvault.NewManagerWrapper().Init(context.TODO(), "qa"); err != nil {
		fmt.Printf("[WARN] init sqb vault error: %v\n", err)
		return
	}

	slsFactory := sls.NewFactory()
	client, err := slsFactory.CreateClient("cn-hangzhou.log.aliyuncs.com")
	if err != nil {
		fmt.Println("create sls client error", err)
		return
	}

	f := &SLSFetcher{
		merge: newMergeService(),
	}
	f.begin = time.Now().Add(-time.Hour * 24)
	f.end = time.Now().Add(-time.Hour * 23)

	cli := &SLSClient{
		readed: make(chan struct{}, 1),
		opt: &SLSClientOption{
			Endpoint:    "cn-hangzhou.log.aliyuncs.com",
			ProjectName: "pay-group",
			StoreName:   "upay-transaction-lindorm",
			Concurrency: 2,
			Query:       "arguments=* and message=\"invoking method start\"",
		},
		begin:  f.begin,
		end:    f.end,
		client: client,
	}
	cli.WithDecoder(&testDecoder{})
	f.clients = append(f.clients, cli)

	var ctx = context.Background()
	var output = make(chan *pb.LogRecord, 100)

	go func() {
		for log := range output {
			fmt.Println(log)
		}
	}()

	err = f.Fetch(ctx, output)
	assert.Nil(t, err)
}
