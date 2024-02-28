package certbot

import (
	"fmt"
	"log"
	"slices"
	"strings"

	uuid "github.com/google/uuid"
	component "github.com/jurooravec/helpa/pkg/component"
	lo "github.com/samber/lo"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

type Domain struct {
	// E.g. `example.com`
	Domain string
	// E.g. `*.example.com`
	Subdomains []string
}

type Input struct {
	CertbotNamespace string
	// E.g `"20 3 * * */6"` for "Every 6th day-of-week at 03:20"
	CertbotCronSchedule string
	// Args passed to `certbot` CLI
	CertbotCmdArgs          string
	CertbotContactEmail     string
	CertbotImagePullSecrets []corev1.LocalObjectReference
	CertbotContainer        corev1.Container
	Domain                  Domain
	// Name of the secret for the TLS certificate
	TlsSecretName string
	// Namespaces where Certbot has the permission to create and update
	// the TLS cert secret
	TlsSecretNamespaces []string
	// Whether to schedule a job that would run immediately after the chart is applied
	// Otherwise, the schedule cronjob will run only its period has run
	RunImmediately bool
}

type Context struct {
	Input

	Id         string
	CertbotCmd string
}

var Component component.ComponentMulti[runtime.Object, Input]

func init() {
	err := error(nil)
	Component, err = component.CreateComponentMulti(
		component.DefMulti[runtime.Object, Input, Context]{
			Name:           "Certbot",
			Template:       `./helmchart/src/certbot/certbot.yaml`,
			TemplateIsFile: true,
			Setup: func(input Input) (Context, error) {
				certbotCmd := genCertbotCmd(CertbotCmdInput{
					Args:             input.CertbotCmdArgs,
					Email:            input.CertbotContactEmail,
					Domain:           input.Domain,
					SecretName:       input.TlsSecretName,
					SecretNamespaces: input.TlsSecretNamespaces,
				})

				return Context{
					Input:      input,
					Id:         randomString(6),
					CertbotCmd: certbotCmd,
				}, nil
			},
			GetInstances: func(input Input, context Context) ([]runtime.Object, error) {
				instances := []runtime.Object{
					&batchv1.CronJob{},
					&corev1.ServiceAccount{},
					&rbacv1.ClusterRole{},
				}
				// Add one-off job
				if input.RunImmediately {
					instances = slices.Insert[[]runtime.Object, runtime.Object](instances, 1, &batchv1.CronJob{})
				}

				// Append role bindings
				for range input.TlsSecretNamespaces {
					instances = append(instances, &rbacv1.RoleBinding{})
				}

				return instances, nil
			},
		})

	if err != nil {
		log.Panic(err)
	}
}

func randomString(length int) string {
	return uuid.NewString()[:length]
}

type CertbotCmdInput struct {
	Args             string
	Email            string
	Domain           Domain
	SecretName       string
	SecretNamespaces []string
}

func genCertbotCmd(input CertbotCmdInput) string {
	doms := []string{input.Domain.Domain}
	doms = append(doms, input.Domain.Subdomains...)
	domsStr := strings.Join(
		lo.FilterMap(doms, func(dom string, _ int) (string, bool) {
			return fmt.Sprintf(`-d "%s"`, dom), dom != ""
		}),
		" ",
	)
	secretsCmds := strings.Join(
		lo.Map(input.SecretNamespaces, func(ns string, _ int) string {
			return "&& " + genCreateSecretCmd(input.SecretName, ns)
		}),
		" ",
	)

	return strings.Join([]string{
		fmt.Sprintf(`certbot %s --email %s %s`, input.Args, input.Email, domsStr),
		fmt.Sprintf(`&& cd /etc/letsencrypt/live/%s %s`, input.Domain.Domain, secretsCmds),
	}, " ")
}

func genCreateSecretCmd(secretName string, namespace string) string {
	return strings.Join([]string{
		fmt.Sprintf(`kubectl delete secret %s -n %s || true`, secretName, namespace),
		fmt.Sprintf(`&& kubectl create secret tls %s -n %s --cert=fullchain.pem --key=privkey.pem`, secretName, namespace),
	}, " ")
}
