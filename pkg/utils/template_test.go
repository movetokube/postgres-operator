package utils

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Template Utils", func() {
	ginkgo.Context("RenderTemplate function", func() {
		var (
			templateContext TemplateContext
			templates       map[string]string
		)

		ginkgo.BeforeEach(func() {
			templateContext = TemplateContext{
				Host:     "localhost",
				Role:     "admin",
				Database: "postgres",
				Password: "secret",
				Hostname: "localhost",
				Port:     "5432",
			}
		})

		ginkgo.When("provided with valid templates", func() {
			ginkgo.BeforeEach(func() {
				templates = map[string]string{
					"simple":            "Host: {{.Host}}",
					"all-fields":        "Host: {{.Host}}, Role: {{.Role}}, DB: {{.Database}}, Password: {{.Password}}",
					"multi-line":        "Connection Info:\n  Host: {{.Host}}\n  Database: {{.Database}}",
					"empty-templ":       "",
					"connection-string": "postgres://{{.Role}}:{{.Password}}@{{.Hostname}}:{{.Port}}/{{.Database}}",
				}
			})

			ginkgo.It("should render all templates correctly", func() {
				result, err := RenderTemplate(templates, templateContext)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(result).To(gomega.HaveLen(5))

				gomega.Expect(string(result["simple"])).To(gomega.Equal("Host: localhost"))
				gomega.Expect(string(result["all-fields"])).To(gomega.Equal("Host: localhost, Role: admin, DB: postgres, Password: secret"))
				gomega.Expect(string(result["multi-line"])).To(gomega.Equal("Connection Info:\n  Host: localhost\n  Database: postgres"))
				gomega.Expect(string(result["empty-templ"])).To(gomega.Equal(""))
				gomega.Expect(string(result["connection-string"])).To(gomega.Equal("postgres://admin:secret@localhost:5432/postgres"))
			})
		})

		ginkgo.When("provided with an invalid template", func() {
			ginkgo.BeforeEach(func() {
				templates = map[string]string{
					"invalid": "Host: {{.Host}}, Invalid: {{.NonExistent}}",
				}
			})

			ginkgo.It("should return an error", func() {
				result, err := RenderTemplate(templates, templateContext)

				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("execute template"))
				gomega.Expect(result).To(gomega.BeNil())
			})
		})

		ginkgo.When("provided with a template with syntax error", func() {
			ginkgo.BeforeEach(func() {
				templates = map[string]string{
					"syntax-error": "Host: {{.Host}, Missing closing bracket",
				}
			})

			ginkgo.It("should return an error", func() {
				result, err := RenderTemplate(templates, templateContext)

				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("parse template"))
				gomega.Expect(result).To(gomega.BeNil())
			})
		})

		ginkgo.When("provided with an empty template map", func() {
			ginkgo.BeforeEach(func() {
				templates = map[string]string{}
			})

			ginkgo.It("should return nil", func() {
				result, err := RenderTemplate(templates, templateContext)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(result).To(gomega.BeNil())
			})
		})

		ginkgo.When("provided with a nil template map", func() {
			ginkgo.It("should return nil", func() {
				result, err := RenderTemplate(nil, templateContext)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(result).To(gomega.BeNil())
			})
		})
	})
})
