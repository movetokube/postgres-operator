package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Random String Utils", func() {
	Context("Generating random strings", func() {
		const testLength = 10

		When("using GetRandomString", func() {
			It("should return a string of the specified length", func() {
				result := GetRandomString(testLength)
				Expect(result).To(HaveLen(testLength))
			})

			It("should only contain valid characters", func() {
				result := GetRandomString(testLength)
				validChars := "^[a-zA-Z0-9]+$"
				Expect(result).To(MatchRegexp(validChars))
			})

			It("should generate different strings on multiple calls", func() {
				result1 := GetRandomString(testLength)
				result2 := GetRandomString(testLength)
				Expect(result1).NotTo(Equal(result2))
			})
		})

		When("using GetSecureRandomString", func() {
			It("should return a string of the specified length", func() {
				result, err := GetSecureRandomString(testLength)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(testLength))
			})

			It("should only contain valid characters", func() {
				result, err := GetSecureRandomString(testLength)
				Expect(err).NotTo(HaveOccurred())
				validChars := "^[a-zA-Z0-9]+$"
				Expect(result).To(MatchRegexp(validChars))
			})

			It("should generate different strings on multiple calls", func() {
				result1, err1 := GetSecureRandomString(testLength)
				result2, err2 := GetSecureRandomString(testLength)
				Expect(err1).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred())
				Expect(result1).NotTo(Equal(result2))
			})

			It("should handle generating strings of different lengths", func() {
				result1, err1 := GetSecureRandomString(5)
				result2, err2 := GetSecureRandomString(15)
				Expect(err1).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred())
				Expect(result1).To(HaveLen(5))
				Expect(result2).To(HaveLen(15))
			})
		})
	})
})
