package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MatchesInstanceAnnotation", func() {
	Context("when filter is defined", func() {
		const filter = "value"

		It("should return false when annotations are nil", func() {
			Expect(MatchesInstanceAnnotation(nil, filter)).To(BeFalse())
		})

		It("should return false when correct key is not present", func() {
			annotations := map[string]string{
				"invalidkey": "value",
			}
			Expect(MatchesInstanceAnnotation(annotations, filter)).To(BeFalse())
		})

		It("should return true when correct key and value match", func() {
			annotations := map[string]string{
				INSTANCE_ANNOTATION: "value",
			}
			Expect(MatchesInstanceAnnotation(annotations, filter)).To(BeTrue())
		})
	})

	Context("when filter is not defined", func() {
		const filter = ""

		It("should return true when annotations are nil", func() {
			Expect(MatchesInstanceAnnotation(nil, filter)).To(BeTrue())
		})

		It("should return true when annotations map is empty", func() {
			annotations := map[string]string{}
			Expect(MatchesInstanceAnnotation(annotations, filter)).To(BeTrue())
		})

		It("should return true when correct key is not present", func() {
			annotations := map[string]string{
				"invalidkey": "value",
			}
			Expect(MatchesInstanceAnnotation(annotations, filter)).To(BeTrue())
		})

		It("should return false when the instance annotation key is present", func() {
			annotations := map[string]string{
				INSTANCE_ANNOTATION: "value",
			}
			Expect(MatchesInstanceAnnotation(annotations, filter)).To(BeFalse())
		})
	})
	Context("Testing case insensitivity", func() {
		It("should match values case-insensitively", func() {
			annotations := map[string]string{
				INSTANCE_ANNOTATION: "VALUE",
			}
			Expect(MatchesInstanceAnnotation(annotations, "value")).To(BeTrue())

			annotations = map[string]string{
				INSTANCE_ANNOTATION: "value",
			}
			Expect(MatchesInstanceAnnotation(annotations, "VALUE")).To(BeTrue())
		})
	})

	Context("Testing with mixed cases", func() {
		It("should match values with mixed cases", func() {
			annotations := map[string]string{
				INSTANCE_ANNOTATION: "MiXeDcAsE",
			}
			Expect(MatchesInstanceAnnotation(annotations, "mixedcase")).To(BeTrue())
			Expect(MatchesInstanceAnnotation(annotations, "MIXEDCASE")).To(BeTrue())
		})
	})
})
