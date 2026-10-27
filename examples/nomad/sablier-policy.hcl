# ACL policy for the Sablier token:
#   nomad acl policy apply -description "Sablier" sablier sablier-policy.hcl
#   nomad acl token create -name sablier -policy sablier -type client
namespace "default" {
  # list-jobs and read-job: discover task groups, read allocations, watch job events.
  # scale-job: change the count of a task group.
  # submit-job: register the job with a new count while a deployment blocks scaling.
  capabilities = ["list-jobs", "read-job", "scale-job", "submit-job"]
}
