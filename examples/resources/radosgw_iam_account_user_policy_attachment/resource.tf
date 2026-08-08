# Attach an AWS-managed policy to an account user.
resource "radosgw_iam_account_user_policy_attachment" "read_only" {
  provider   = radosgw.account_root
  user       = radosgw_iam_account_user.developer.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
}
