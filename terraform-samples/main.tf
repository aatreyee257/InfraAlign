resource "aws_s3_bucket" "mybucket" {
  bucket = "my-tf-demo-bucket"
}

resource "aws_s3_bucket_server_side_encryption_configuration" "mybucket_encryption" {
  bucket = aws_s3_bucket.mybucket.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}