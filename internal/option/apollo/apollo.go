package apollo

import (
	"go.uber.org/zap"
	"strings"

	"github.com/apolloconfig/agollo/v4"
	apolloConfig "github.com/apolloconfig/agollo/v4/env/config"
	"github.com/jacexh/gopkg/config"
)

type apollo struct {
	client agollo.Client
	opt    *options
}

// Option is apollo option
type Option func(*options)

type options struct {
	appid          string
	secret         string
	cluster        string
	endpoint       string
	namespace      string
	isBackupConfig bool
	backupPath     string
	logger         *zap.Logger
}

// WithAppID with apollo config app id
func WithAppID(appID string) Option {
	return func(o *options) {
		o.appid = appID
	}
}

// WithCluster with apollo config cluster
func WithCluster(cluster string) Option {
	return func(o *options) {
		o.cluster = cluster
	}
}

// WithEndpoint with apollo config conf server ip
func WithEndpoint(endpoint string) Option {
	return func(o *options) {
		o.endpoint = endpoint
	}
}

// WithEnableBackup with apollo config enable backup config
func WithEnableBackup() Option {
	return func(o *options) {
		o.isBackupConfig = true
	}
}

// WithDisableBackup with apollo config enable backup config
func WithDisableBackup() Option {
	return func(o *options) {
		o.isBackupConfig = false
	}
}

// WithSecret with apollo config app secret
func WithSecret(secret string) Option {
	return func(o *options) {
		o.secret = secret
	}
}

// WithNamespace with apollo config namespace name
func WithNamespace(name string) Option {
	return func(o *options) {
		o.namespace = name
	}
}

// WithBackupPath with apollo config backupPath
func WithBackupPath(backupPath string) Option {
	return func(o *options) {
		o.backupPath = backupPath
	}
}

// WithBackupPath with apollo config backupPath
func WithLogger(log *zap.Logger) Option {
	return func(o *options) {
		o.logger = log
	}
}

func NewSource(opts ...Option) config.Source {
	op := options{}
	for _, o := range opts {
		o(&op)
	}
	client, err := agollo.StartWithConfig(func() (*apolloConfig.AppConfig, error) {
		return &apolloConfig.AppConfig{
			AppID:            op.appid,
			Cluster:          op.cluster,
			NamespaceName:    op.namespace,
			IP:               op.endpoint,
			IsBackupConfig:   op.isBackupConfig,
			Secret:           op.secret,
			BackupConfigPath: op.backupPath,
		}, nil
	})
	if err != nil {
		panic(err)
	}
	return &apollo{client: client, opt: &op}
}

func format(ns string) string {
	arr := strings.Split(ns, ".")
	if len(arr) <= 1 {
		return "json"
	}

	return arr[len(arr)-1]
}

func (e *apollo) load() []*config.KeyValue {
	kv := make([]*config.KeyValue, 0)
	namespaces := strings.Split(e.opt.namespace, ",")

	for _, ns := range namespaces {
		value, err := e.client.GetConfigCache(ns).Get("content")
		if err != nil {
			e.opt.logger.Warn("apollo get config failed", zap.Error(err))
		}
		// serialize the namespace content KeyValue into bytes.
		kv = append(kv, &config.KeyValue{
			Key:    ns,
			Value:  []byte(value.(string)),
			Format: format(ns),
		})
	}

	return kv
}

func (e *apollo) Load() (kv []*config.KeyValue, err error) {
	return e.load(), nil
}

func (e *apollo) Watch() (config.Watcher, error) {
	w, err := newWatcher(e)
	if err != nil {
		return nil, err
	}
	return w, nil
}
