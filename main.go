package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
)

func main() {
	containerWorkspace, _ := os.LookupEnv("CONTAINER_WORKSPACE")
	localWorkspace, _ := os.LookupEnv("LOCAL_WORKSPACE")

	file := "docker-compose.yml"
	if _, err := os.Stat(file); err != nil {
		file = "docker-compose.yaml"
	}

	args := []string{}
	mode := 0
	for _, arg := range os.Args[1:] {
		switch mode {
		case 0:
			switch {
			case arg == "-f" || arg == "--file":
				mode = 1
			case strings.HasPrefix(arg, "-"):
				args = append(args, arg)
			default:
				args = append(args, arg)
				mode = -1
			}
		case 1:
			file = arg
			mode = 0
		default:
			args = append(args, arg)
		}
	}

	realpath := func(path string) string {
		out, err := exec.Command("realpath", "-m", "--relative-base="+containerWorkspace, path).Output()
		if err != nil {
			panic(err)
		}

		path = strings.TrimSpace(string(out))

		if strings.HasPrefix(path, "/") {
			fmt.Fprintln(os.Stderr, "bind mount can only in workspace folder: "+path)
			os.Exit(1)
		}

		return filepath.Join(localWorkspace, path)
	}

	defer func() {
		if err := recover(); err != nil {
			exitCode := execCompose(os.Args[1:]...)
			os.Exit(exitCode)
		}
	}()

	text, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}

	var data interface{}

	if err := yaml.Unmarshal(text, &data); err != nil {
		panic(err)
	}

	compose := reflect.ValueOf(data)
	services := compose.MapIndex(reflect.ValueOf("services")).Elem()

	for _, key := range services.MapKeys() {
		service := services.MapIndex(key).Elem()
		volumes := service.MapIndex(reflect.ValueOf("volumes")).Elem()

		for i := 0; i < volumes.Len(); i++ {
			volume := volumes.Index(i).Elem()

			switch volume.Kind() {
			case reflect.String:
				if v := strings.SplitN(volume.String(), ":", 2); len(v) == 2 {
					if t := compose.MapIndex(reflect.ValueOf("volumes")); t.IsValid() {
						if t.Elem().Kind() != reflect.Map || t.Elem().MapIndex(reflect.ValueOf(v[0])).IsValid() {
							continue
						}
					}
					path := realpath(v[0])
					volumes.Index(i).Set(reflect.ValueOf(path + ":" + v[1]))
				}

			case reflect.Map:
				vtype := volume.MapIndex(reflect.ValueOf("type")).Elem()
				if vtype.String() == "bind" {
					source := volume.MapIndex(reflect.ValueOf("source")).Elem()
					path := realpath(source.String())
					volume.SetMapIndex(reflect.ValueOf("source"), reflect.ValueOf(path))
				}
			}
		}
	}

	f, err := os.CreateTemp(os.TempDir(), "docker-compose-*.yml")
	if err != nil {
		panic(err)
	}
	err = yaml.NewEncoder(f).Encode(data)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		panic(err)
	}

	args = append([]string{"--file", f.Name()}, args...)
	exitCode := execCompose(args...)
	os.Remove(f.Name())
	os.Exit(exitCode)
}

func execCompose(args ...string) int {
	cmd := exec.Command("/usr/local/bin/docker-compose", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 256
	}
	return cmd.ProcessState.ExitCode()
}
