# Attach an AWS-managed policy to a role (inline policies use radosgw_iam_role_policy).
resource "radosgw_iam_role_policy_attachment" "read_only" {
  role       = radosgw_iam_role.app.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
}
