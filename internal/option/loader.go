package option

import (
	"errors"
	"fmt"
	"github.com/jacexh/gopkg/config"
	"github.com/jacexh/gopkg/config/env"
	"github.com/jacexh/gopkg/config/file"
	"github.com/wosai/havok/internal/option/apollo"
	"os"
	"path/filepath"
	"runtime"
)

var (
	configFileType = "yml"
	configName     = "config"
	// searchInPaths 配置文件查找目录
	searchInPaths []string

	// environmentVariablesPrefix 项目环境变量前缀
	environmentVariablesPrefix = "HAVOK"
	// environmentVariableProfile 项目profile的环境变量名称
	environmentVariableProfile = environmentVariablesPrefix + "_PROJECT_PROFILE"
	Conf                       *Option
)

// SetConfigFileType 配置文件类型
func SetConfigFileType(t string) {
	if t != "" {
		configFileType = t
	}
}

// SetConfigName 配置名称
func SetConfigName(n string) {
	if n != "" {
		configName = n
	}
}

// AddConfigPath 添加配置文件目录
func AddConfigPath(path string) {
	if path == "" {
		return
	}
	if filepath.IsAbs(path) {
		searchInPaths = append(searchInPaths, filepath.Clean(path))
		return
	}
	fp, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	searchInPaths = append(searchInPaths, fp)
}

func HomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func getConfigName() string {
	profile := os.Getenv(environmentVariableProfile)
	if profile == "" {
		return configName
	}
	return fmt.Sprintf("%s_%s", configName, profile)
}

func findInDir(dir string, file string) string {
	fp := filepath.Join(dir, file)
	fi, err := os.Stat(fp)
	if err == nil && !fi.IsDir() {
		return fp
	}
	return ""
}

func findConfigFile() string {
	fp := fmt.Sprintf("%s.%s", getConfigName(), configFileType)
	for _, d := range searchInPaths {
		if p := findInDir(d, fp); p != "" {
			return p
		}
	}
	panic(errors.New("cannot find the config file"))
}

func findApolloConfigs() (opts []apollo.Option) {

	configServerURL := os.Getenv("APOLLO_HOST")
	if configServerURL == "" {
		return nil
	}

	configAppID := os.Getenv("APOLLO_APPID")

	configCluster := os.Getenv("APOLLO_CLUSTER")
	if configCluster == "" {
		configCluster = "default"
	}

	configNamespace := os.Getenv("APOLLO_NAMESPACE")
	if configNamespace == "" {
		configNamespace = "config.yml"
	}

	opts = append(opts, apollo.WithEndpoint(configServerURL))
	opts = append(opts, apollo.WithAppID(configAppID))
	opts = append(opts, apollo.WithCluster(configCluster))
	opts = append(opts, apollo.WithNamespace(configNamespace))
	opts = append(opts, apollo.WithEnableBackup())
	return opts
}

func MustLoadConfig() *Option {
	f := findConfigFile()
	var source []config.Source

	// file --> apollo --> env
	source = append(source,
		file.NewSource(f),
		apollo.NewSource(findApolloConfigs()...),
		env.NewSource(environmentVariablesPrefix),
	)

	c := config.New(
		config.WithSource(source...),
	)

	err := c.Load()
	if err != nil {
		panic(err)
	}

	opt := new(Option)
	err = c.Scan(opt)
	if err != nil {
		panic(err)
	}
	return opt
}

//func init() {
//	AddConfigPath("./conf")
//	p, err := os.Getwd()
//	if err != nil {
//		panic("error path" + err.Error())
//	}
//	basePath := strings.SplitAfter(p, "stress-platform")[0]
//	AddConfigPath(path.Join(basePath, "/conf"))
//	Conf = MustLoadConfig()
//}
