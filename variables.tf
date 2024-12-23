variable "rds_cluster_identifier" {
  description = "The identifier of the RDS cluster, used only for setting up event bridge and tf resources naming"
}

variable "tags" {
  description = "A map of tags to add to all resources"
  type        = map(string)
  default     = {}
}

variable "push_tags" {
  description = "Tags to be pushed to the new scaled read replica"
  type        = map(string)
  default     = {}
}

variable "do_not_creat_event_bridge" {
  description = "If set to true, the event bridge rule will not be created"
  type        = bool
  default     = false
}
