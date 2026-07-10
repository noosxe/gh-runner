# ConnectRPC Protocols

This document defines the Protobuf schemas used for binary communication between the Vite/React frontend and the Go backend using **ConnectRPC**.

## `api.proto`

```protobuf
syntax = "proto3";

package supervisor.v1;

option go_package = "github.com/noosxe/gh-runner/src/internal/pb/supervisor/v1";

// ----------------------------------------
// Authentication Service
// ----------------------------------------

service AuthService {
  // Setup the initial local administrator account
  rpc SetupAdmin (SetupAdminRequest) returns (SetupAdminResponse);
  
  // Login with local credentials to receive a session token
  rpc Login (LoginRequest) returns (LoginResponse);
  
  // Verify current session
  rpc GetSession (GetSessionRequest) returns (GetSessionResponse);
}

message SetupAdminRequest {
  string username = 1;
  string password = 2;
}

message SetupAdminResponse {
  bool success = 1;
}

message LoginRequest {
  string username = 1;
  string password = 2;
}

message LoginResponse {
  string token = 1;
}

message GetSessionRequest {}

message GetSessionResponse {
  string username = 1;
  bool is_admin = 2;
}

// ----------------------------------------
// Pool Management Service
// ----------------------------------------

service PoolService {
  rpc ListPools (ListPoolsRequest) returns (ListPoolsResponse);
  rpc CreatePool (CreatePoolRequest) returns (CreatePoolResponse);
  rpc UpdatePool (UpdatePoolRequest) returns (UpdatePoolResponse);
  rpc DeletePool (DeletePoolRequest) returns (DeletePoolResponse);
}

message Pool {
  int64 id = 1;
  string name = 2;
  string provider = 3;
  string repository_url = 4;
  int32 min_idle_runners = 5;
  int32 max_concurrency = 6;
  string labels = 7;
  string runner_image = 8;
  bool allow_docker = 9;
  
  RenovateConfig renovate = 10;
  
  // Runtime stats
  int32 active_runners = 11;
  int32 idle_runners = 12;
}

message RenovateConfig {
  bool enabled = 1;
  string cron_schedule = 2;
  string image = 3;
}

message ListPoolsRequest {}

message ListPoolsResponse {
  repeated Pool pools = 1;
}

message CreatePoolRequest {
  Pool pool = 1;
}

message CreatePoolResponse {
  Pool pool = 1;
}

message UpdatePoolRequest {
  Pool pool = 1;
}

message UpdatePoolResponse {
  Pool pool = 1;
}

message DeletePoolRequest {
  int64 id = 1;
}

message DeletePoolResponse {
  bool success = 1;
}

// ----------------------------------------
// Analytics & History Service
// ----------------------------------------

service AnalyticsService {
  rpc GetJobHistory (GetJobHistoryRequest) returns (GetJobHistoryResponse);
  rpc GetSystemStats (GetSystemStatsRequest) returns (GetSystemStatsResponse);
}

message JobRecord {
  int64 id = 1;
  int64 pool_id = 2;
  string runner_name = 3;
  string status = 4;
  string queued_at = 5;
  string started_at = 6;
  string completed_at = 7;
}

message GetJobHistoryRequest {
  int64 pool_id = 1; // 0 for all
  int32 limit = 2;
  int32 offset = 3;
}

message GetJobHistoryResponse {
  repeated JobRecord jobs = 1;
  int32 total_count = 2;
}

message GetSystemStatsRequest {}

message GetSystemStatsResponse {
  int32 total_active_runners = 1;
  int32 total_idle_runners = 2;
  double average_queue_time_seconds = 3;
}
```
