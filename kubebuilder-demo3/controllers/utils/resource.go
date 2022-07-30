package utils

import (
	"bytes"
	"fmt"
	"github.com/kubebuilder-demo3/api/v1beta1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	//"os"
	//"os/exec"
	//"path/filepath"
	//"strings"
	"text/template"
)

//
//func GetAppPath() string {
//	file, _ := exec.LookPath(os.Args[0])
//	path, _ := filepath.Abs(file)
//	index := strings.LastIndex(path, string(os.PathSeparator))
//
//	fmt.Println(path[:index])
//	return path[:index]
//}

func parseTemplate(templateName string, app *v1beta1.Lezao) []byte {

	//tmpl, err := template.ParseFiles(GetAppPath() + "/controllers/template/" + templateName + ".yml")
	tmpl, err := template.ParseFiles("controllers/template/" + templateName + ".yml")
	if err != nil {
		panic(err)
	}
	b := new(bytes.Buffer)
	err = tmpl.Execute(b, app)
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

func NewDeployment(app *v1beta1.Lezao) *appv1.Deployment {
	d := &appv1.Deployment{}
	fmt.Println("================pre--- ummarshal deployment==============")
	err := yaml.Unmarshal(parseTemplate("deployment", app), d)
	fmt.Println("================post---in utils dir, ummarshal deployment==============")
	if err != nil {
		panic(err)
	}
	return d
}

func NewIngress(app *v1beta1.Lezao) *netv1.Ingress {
	i := &netv1.Ingress{}
	err := yaml.Unmarshal(parseTemplate("ingress", app), i)
	if err != nil {
		panic(err)
	}
	return i
}

func NewService(app *v1beta1.Lezao) *corev1.Service {
	s := &corev1.Service{}
	err := yaml.Unmarshal(parseTemplate("service", app), s)
	if err != nil {
		panic(err)
	}
	return s
}
