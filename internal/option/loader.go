package option

import (
	"errors"
	"fmt"
	"github.com/jacexh/multiconfig"
	"github.com/wosai/havok/pkg/apollo"
	"io"
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
	fmt.Println(searchInPaths)
	for _, d := range searchInPaths {
		if p := findInDir(d, fp); p != "" {
			return p
		}
	}
	panic(errors.New("cannot find the config file"))
}

func LoadFromApollo(conf interface{}, host, appid, namespace, keyname string) {
	if host == "" || appid == "" || namespace == "" {
		panic("host/appID/namespace not empty")
	}

	client, err := apollo.NewClient(apollo.WithUrl(host), apollo.WithAppId(appid), apollo.WithNamespace(namespace))
	if err != nil {
		panic(err)
	}
	c, ok := client.GetConfig(keyname)
	if !ok {
		panic(fmt.Sprintf("%s in %s/%s is empty", keyname, appid, namespace))
	}
	if _, err := os.Stat("./" + keyname); os.IsExist(err) {
		os.Remove("./" + keyname)
	}
	file, err := os.Create("./" + keyname)
	if err != nil {
		panic(err)
	}
	_, err = io.WriteString(file, fmt.Sprintf("%v", c))
	if err != nil {
		panic(err)
	}
	loader := multiconfig.NewWithPathAndEnvPrefix("./"+keyname, environmentVariablesPrefix)
	loader.MustLoad(conf)
}

func LoadFromFile(conf interface{}, conf_path string) {
	AddConfigPath(conf_path)
	//p, err := os.Getwd()
	//if err != nil {
	//	panic("error path" + err.Error())
	//}
	//basePath := strings.SplitAfter(p, "havok")[0]
	//AddConfigPath(path.Join(basePath, "/conf"))
	f := findConfigFile()
	loader := multiconfig.NewWithPathAndEnvPrefix(f, environmentVariablesPrefix)
	loader.MustLoad(conf)
}
