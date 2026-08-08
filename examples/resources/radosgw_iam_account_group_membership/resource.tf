# Manages the group's FULL member set (exclusive).
resource "radosgw_iam_account_group_membership" "engineering" {
  provider = radosgw.account_root
  group    = radosgw_iam_account_group.engineering.name
  users = [
    radosgw_iam_account_user.developer.name,
    radosgw_iam_account_user.analyst.name,
  ]
}
