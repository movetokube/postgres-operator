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

		When("using GeneratePassword with default configuration", func() {
			It("should mimic legacy behavior (15 chars, alphanumeric)", func() {
				// verify default behavior (15 chars, alphanumeric) when config is empty
				config := PostgresPassPolicy{}
				result, err := GeneratePassword(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(15))
				Expect(result).To(MatchRegexp("^[a-zA-Z0-9]+$"))
			})
		})

		When("using GeneratePassword with specific length", func() {
			It("should return a string of the specified length", func() {
				config := PostgresPassPolicy{Length: 20}
				result, err := GeneratePassword(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(20))
			})

			It("should remain alphanumeric if no complexity requirements are set", func() {
				// verify that the default pool remains alphanumeric when MinSpecial is 0
				config := PostgresPassPolicy{Length: 50}
				result, err := GeneratePassword(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(50))
				// Should strictly be alphanumeric
				Expect(result).To(MatchRegexp("^[a-zA-Z0-9]+$"))
				// Should NOT contain special characters
				Expect(result).NotTo(MatchRegexp(`[!@#$%^&*]`))
			})

			It("should validate length vs requirements", func() {
				// verify error when length is less than sum of minimum requirements (20 > 10)
				config := PostgresPassPolicy{
					Length:     10,
					MinLower:   5,
					MinUpper:   5,
					MinNumeric: 5,
					MinSpecial: 5,
				}
				_, err := GeneratePassword(config)
				Expect(err).To(HaveOccurred())
			})
		})

		When("using GeneratePassword with complexity requirements", func() {
			It("should satisfy minimum character counts", func() {
				config := PostgresPassPolicy{
					Length:     50,
					MinLower:   5,
					MinUpper:   5,
					MinNumeric: 5,
					MinSpecial: 5,
				}
				// Generate multiple times to be sure
				for i := 0; i < 10; i++ {
					result, err := GeneratePassword(config)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(MatchRegexp(`[a-z].*[a-z].*[a-z].*[a-z].*[a-z]`))
					Expect(result).To(MatchRegexp(`[A-Z].*[A-Z].*[A-Z].*[A-Z].*[A-Z]`))
					Expect(result).To(MatchRegexp(`[0-9].*[0-9].*[0-9].*[0-9].*[0-9]`))
					// verify special character count manually as they require escaping in regex
					specialCount := 0
					for _, c := range result {
						for _, s := range "!@#$%^&*" {
							if c == s {
								specialCount++
								break
							}
						}
					}
					Expect(specialCount).To(BeNumerically(">=", 5))
				}
			})
		})

		When("using GeneratePassword with exclusion", func() {
			It("should not contain excluded characters", func() {
				config := PostgresPassPolicy{
					Length:       100,
					ExcludeChars: "abc123!@#",
				}
				result, err := GeneratePassword(config)
				Expect(err).NotTo(HaveOccurred())
				for _, c := range result {
					Expect(string(c)).NotTo(BeElementOf("a", "b", "c", "1", "2", "3", "!", "@", "#"))
				}
			})

			It("should error if exclusion leaves no characters", func() {
				// verify error when all characters are excluded
				config := PostgresPassPolicy{
					Length:       10,
					ExcludeChars: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890!@#$%^&*",
				}
				_, err := GeneratePassword(config)
				Expect(err).To(HaveOccurred())
			})
		})

		When("using GeneratePassword with EnsureFirstLetter", func() {
			It("should make sure the password starts with a letter", func() {
				// set high numeric/special counts to increase probability of non-letter first char,
				// but ensure at least one letter exists for swapping
				config := PostgresPassPolicy{
					Length:            10,
					MinNumeric:        5,
					MinSpecial:        2,
					MinLower:          1,
					EnsureFirstLetter: true,
				}

				for i := 0; i < 50; i++ {
					result, err := GeneratePassword(config)
					Expect(err).NotTo(HaveOccurred())
					firstChar := rune(result[0])
					isLetter := (firstChar >= 'a' && firstChar <= 'z') || (firstChar >= 'A' && firstChar <= 'Z')
					Expect(isLetter).To(BeTrue(), "Expected first char %c to be a letter", firstChar)
				}
			})

			It("should error if no letters are available to start with", func() {
				// verify error when numeric requirements consume entire length, leaving no room for a letter
				config := PostgresPassPolicy{
					Length:            10,
					MinNumeric:        10,
					EnsureFirstLetter: true,
				}
				_, err := GeneratePassword(config)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
