package config

import "testing"

func TestR2OverridesLocalDefaults(t *testing.T) {
	t.Setenv("STORAGE_PROVIDER", "r2")
	t.Setenv("STORAGE_ENDPOINT", "http://minio:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "minioadmin")
	t.Setenv("STORAGE_SECRET_KEY", "minioadmin")
	t.Setenv("R2_ENDPOINT", "")
	t.Setenv("R2_ACCOUNT_ID", "account-test")
	t.Setenv("R2_ACCESS_KEY_ID", "r2-key")
	t.Setenv("R2_SECRET_ACCESS_KEY", "r2-secret")
	t.Setenv("R2_BUCKET_NAME", "assets")
	t.Setenv("R2_PUBLIC_URL", "https://assets.example.com/")
	c := Load()
	if c.Storage.Endpoint != "https://account-test.r2.cloudflarestorage.com" || c.Storage.Region != "auto" || !c.Storage.PathStyle || c.Storage.Bucket != "assets" || c.Storage.AccessKeyID != "r2-key" || c.Storage.SecretAccessKey != "r2-secret" || c.Storage.PublicBaseURL != "https://assets.example.com" {
		t.Fatal("R2 configuration did not override local storage defaults")
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Storage.Endpoint = "http://minio:9000"
	if c.Validate() == nil {
		t.Fatal("R2 accepted an insecure local endpoint")
	}
	c.Storage.Endpoint = "https://account-test.r2.cloudflarestorage.com"
	c.Storage.PublicBaseURL = ""
	if c.Validate() == nil {
		t.Fatal("public asset storage accepted missing public URL")
	}
}

func TestMinIODoesNotUseR2Credentials(t *testing.T) {
	t.Setenv("STORAGE_PROVIDER", "minio")
	t.Setenv("STORAGE_ACCESS_KEY", "local-key")
	t.Setenv("R2_ACCESS_KEY_ID", "r2-key")
	if Load().Storage.AccessKeyID != "local-key" {
		t.Fatal("R2 changed MinIO configuration")
	}
}
