
```go
var (
	projectID = os.Getenv("PORJECT_ID")
	keyRing   = os.Getenv("KEY_RING")
	keyName   = os.Getenv("KEY_NAME")
)
var fullKeyName = fmt.Sprintf("projects/%s/locations/global/keyRings/%s/cryptoKeys/%s", projectID, keyRing, keyName)
pwd, err := cloudkey.EncryptSymmetric(fullKeyName, "12345")
if err != nil {
	fmt.Println(err.Error())
}
err = cloudkey.DecryptSymmetric(fullKeyName, pwd)
if err != nil {
	fmt.Println(err.Error())
}
fmt.Println(pwd)
```