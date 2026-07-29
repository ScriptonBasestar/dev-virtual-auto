#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

check_state() {
  local state=$1
  ruby -rpsych - "$state" <<'RUBY'
path = ARGV.fetch(0)
abort_with = ->(message) { warn "FAIL: #{message}"; exit 1 }

mapping = lambda do |node, name|
  abort_with.call("#{name} must be a mapping") unless node.is_a?(Psych::Nodes::Mapping)
  node.children.each_slice(2).each_with_object({}) do |(key, value), entries|
    abort_with.call("#{name} contains a non-scalar key") unless key.is_a?(Psych::Nodes::Scalar)
    abort_with.call("#{name} contains duplicate key #{key.value}") if entries.key?(key.value)
    entries[key.value] = value
  end
end

scalar = lambda do |entries, key, name|
  node = entries[key]
  abort_with.call("missing #{name}.#{key}") unless node.is_a?(Psych::Nodes::Scalar)
  node.value
end

document = Psych.parse_file(path)
abort_with.call("empty YAML document") if document.nil?
root = mapping.call(document.root, "state")
run = mapping.call(root["run"], "run")
evaluation = mapping.call(root["evaluation"], "evaluation")
revisions = mapping.call(root["revisions"], "revisions")
sources = mapping.call(root["sources"], "sources")
operations = mapping.call(root["operations"], "operations")
authority = mapping.call(root["authority"], "authority")

primary_owner = scalar.call(run, "primary_owner", "run")
owner_route = scalar.call(run, "owner_route", "run")
mode = scalar.call(run, "mode", "run")
next_prompt = scalar.call(root, "next_prompt", "state")
abort_with.call("missing run.predecessor_run_id") unless run.key?("predecessor_run_id")

packages_root = scalar.call(sources, "packages_root", "sources")
abort_with.call("sources.packages_root must be an absolute literal input") unless packages_root.start_with?("/")
packages_head = scalar.call(revisions, "packages_head", "revisions")
abort_with.call("revisions.packages_head must be a Git SHA") unless packages_head.match?(/\A[0-9a-f]{40}\z/)
packages_dirty_hash = scalar.call(revisions, "packages_dirty_hash", "revisions")
abort_with.call("revisions.packages_dirty_hash must be a SHA-256") unless packages_dirty_hash.match?(/\A[0-9a-f]{64}\z/)
protected_dirty_paths = revisions["packages_protected_dirty_paths"]
abort_with.call("revisions.packages_protected_dirty_paths must be a non-empty scalar sequence") unless protected_dirty_paths.is_a?(Psych::Nodes::Sequence) && !protected_dirty_paths.children.empty? && protected_dirty_paths.children.all? { |node| node.is_a?(Psych::Nodes::Scalar) && !node.value.empty? }
projection = scalar.call(sources, "config_projection", "sources")
abort_with.call("invalid sources.config_projection") unless %w[active missing].include?(projection)
config_required = scalar.call(operations, "config_invocation_required", "operations")
abort_with.call("operations.config_invocation_required must be true or false") unless %w[true false].include?(config_required)
abort_with.call("missing config projection must block only an invocation-requiring operation") if projection == "missing" && config_required == "true" && scalar.call(run, "status", "run") != "blocked"
authority_scope = scalar.call(authority, "scope", "authority")
authority_approved = scalar.call(authority, "approved", "authority")
abort_with.call("invalid authority.scope") unless %w[numbered_stage post_cycle_qa].include?(authority_scope)
abort_with.call("authority.approved must be true or false") unless %w[true false].include?(authority_approved)
commands = authority["commands"]
abort_with.call("authority.commands must be a sequence") unless commands.is_a?(Psych::Nodes::Sequence)
allowed_effects = {
  "dva provision" => "provisions_public_directories",
  "dva up full-stack" => "starts_full_stack_services",
  "dva down full-stack" => "stops_cycle_started_full_stack_services"
}
commands.children.each do |node|
  entry = mapping.call(node, "authority.commands entry")
  abort_with.call("authority.commands entry keys differ") unless entry.keys.sort == %w[command side_effect]
  command = scalar.call(entry, "command", "authority.commands entry")
  effect = scalar.call(entry, "side_effect", "authority.commands entry")
  abort_with.call("authority command has undeclared side effect") unless allowed_effects[command] == effect
end
if authority_approved == "true"
  abort_with.call("approved authority requires post_cycle_qa scope") unless authority_scope == "post_cycle_qa"
  abort_with.call("approved authority requires exact command authority") if commands.children.empty?
end

owners = %w[plugin local_setup target environment no_change]
routes = %w[skill prompt dva_tool target_project environment no_change]
abort_with.call("invalid run.primary_owner: #{primary_owner}") unless owners.include?(primary_owner)
abort_with.call("invalid run.owner_route: #{owner_route}") unless routes.include?(owner_route)
abort_with.call("invalid run.mode: #{mode}") unless %w[continuous step].include?(mode)

route_contract = {
  "skill" => ["plugin", "30-improve-skill.md"],
  "prompt" => ["local_setup", "40-improve-prompts.md"],
  "dva_tool" => ["plugin", "45-improve-dva-tool.md"],
  "target_project" => ["target", "50-apply-to-project.md"],
  "environment" => ["environment", "60-evaluate.md"],
  "no_change" => ["no_change", "50-apply-to-project.md"]
}
expected_owner, expected_prompt = route_contract.fetch(owner_route)
abort_with.call("owner route #{owner_route} cannot map to primary owner #{primary_owner}") unless primary_owner == expected_owner

stages = mapping.call(root["stages"], "stages")
stage_status = ->(id) { scalar.call(mapping.call(stages[id], "stages.#{id}"), "status", "stages.#{id}") }
allowed_prompts =
  if stage_status.call("70") == "complete"
    %w[00-start-cycle.md post-cycle-qa.md none]
  elsif stage_status.call("60") == "complete"
    [expected_prompt, "70-feedback.md"]
  elsif stage_status.call("50") == "complete"
    ["60-evaluate.md"]
  else
    [expected_prompt]
  end
abort_with.call("owner route #{owner_route} cannot select next prompt #{next_prompt}") unless allowed_prompts.include?(next_prompt)

version = scalar.call(evaluation, "version", "evaluation")
manifest_hash = scalar.call(evaluation, "case_manifest_hash", "evaluation")
case_ids = evaluation["case_ids"]
abort_with.call("missing evaluation.version") if version.empty?
abort_with.call("missing evaluation.case_manifest_hash") if manifest_hash.empty?
abort_with.call("missing evaluation.case_ids") unless case_ids.is_a?(Psych::Nodes::Sequence) && !case_ids.children.empty?
abort_with.call("missing evaluation.forward_requests_hash") unless evaluation.key?("forward_requests_hash")

puts "PASS: valid DVA state contract (#{owner_route} -> #{primary_owner})"
RUBY
}

check_evaluation() {
  local state=$1
  local run_dir=$2
  ruby -rpsych -rdigest - "$state" "$run_dir" "$ROOT/ref-evaluation.md" <<'RUBY'
state_path, run_dir, reference_path = ARGV

def mismatch(message)
  warn "FAIL: evaluation_contract_mismatch: #{message}"
  exit 2
end

def mapping(node, context)
  mismatch("#{context} must be a mapping") unless node.is_a?(Psych::Nodes::Mapping)
  node.children.each_slice(2).each_with_object({}) do |(key, value), result|
    mismatch("#{context} contains a non-scalar key") unless key.is_a?(Psych::Nodes::Scalar)
    mismatch("#{context} contains duplicate key #{key.value}") if result.key?(key.value)
    result[key.value] = value
  end
end

def exact_keys(entries, expected, context)
  actual = entries.keys.sort
  mismatch("#{context} keys must be #{expected.sort.join(',')}") unless actual == expected.sort
end

def scalar(entries, key, context)
  node = entries[key]
  mismatch("missing #{context}.#{key}") unless node.is_a?(Psych::Nodes::Scalar)
  mismatch("empty #{context}.#{key}") if node.value.to_s.empty?
  node.value
end

def scalar_sequence(node, context)
  mismatch("#{context} must be a sequence") unless node.is_a?(Psych::Nodes::Sequence)
  node.children.map.with_index do |child, index|
    mismatch("#{context}[#{index}] must be a scalar") unless child.is_a?(Psych::Nodes::Scalar)
    mismatch("#{context}[#{index}] is empty") if child.value.to_s.empty?
    child.value
  end
end

def document(path, context)
  parsed = Psych.parse_file(path)
  mismatch("#{context} is empty") if parsed.nil?
  mapping(parsed.root, context)
rescue Psych::SyntaxError
  mismatch("#{context} YAML is malformed")
end

reference = File.binread(reference_path)
match = reference.match(%r{<!-- evaluation-manifest:start -->\n```yaml\n(.*?)```\n<!-- evaluation-manifest:end -->}m)
mismatch("canonical manifest is missing") if match.nil?
manifest_bytes = match[1]
manifest_hash = Digest::SHA256.hexdigest(manifest_bytes)
declared_hash = reference.match(/<!-- evaluation-manifest:sha256=([0-9a-f]{64}) -->/)&.captures&.first
mismatch("canonical manifest SHA-256 is missing or differs") unless declared_hash == manifest_hash
manifest = Psych.safe_load(manifest_bytes, aliases: false)
mismatch("canonical manifest must be a mapping") unless manifest.is_a?(Hash)
mismatch("canonical manifest keys must be cases,version") unless manifest.keys.sort == %w[cases version]
mismatch("canonical manifest version must be dva-routing-v1") unless manifest["version"] == "dva-routing-v1"

expected_cases = [
  ["config-schema-ownership", "config_schema"],
  ["provision-safety", "provision"],
  ["root-workers-lifecycle-ownership", "lifecycle_boundary"],
  ["subproject-engine", "subproject"],
  ["subproject-workers", "subproject"],
  ["subproject-transformer", "subproject"],
  ["subproject-e2e", "subproject"],
  ["compose-profiles", "compose_profiles"],
  ["health-runtime-truth", "runtime_truth"],
  ["no-change", "no_change"]
]
mismatch("canonical manifest cases must be an ordered sequence") unless manifest["cases"].is_a?(Array)
actual_cases = manifest["cases"].map do |entry|
  mismatch("canonical manifest case must be a mapping") unless entry.is_a?(Hash) && entry.keys.sort == %w[id surface]
  [entry["id"], entry["surface"]]
end
mismatch("canonical manifest order or coverage changed") unless actual_cases == expected_cases
case_ids = expected_cases.map(&:first)

state = document(state_path, "state")
evaluation = mapping(state["evaluation"], "evaluation")
version = scalar(evaluation, "version", "evaluation")
state_ids = scalar_sequence(evaluation["case_ids"], "evaluation.case_ids")
state_manifest_hash = scalar(evaluation, "case_manifest_hash", "evaluation")
state_forward_hash = scalar(evaluation, "forward_requests_hash", "evaluation")
mismatch("evaluation.version differs") unless version == "dva-routing-v1"
mismatch("evaluation.case_ids differ") unless state_ids == case_ids
mismatch("evaluation.case_manifest_hash differs") unless state_manifest_hash == manifest_hash

forward_path = File.join(run_dir, "forward-requests.md")
mismatch("frozen forward-requests.md is missing") unless File.file?(forward_path)
forward_bytes = File.binread(forward_path)
forward_hash = Digest::SHA256.hexdigest(forward_bytes)
mismatch("evaluation.forward_requests_hash differs") unless state_forward_hash == forward_hash
forward = document(forward_path, "forward requests")
exact_keys(forward, %w[version case_manifest_hash requests], "forward requests")
mismatch("forward request version differs") unless scalar(forward, "version", "forward requests") == version
mismatch("forward request manifest hash differs") unless scalar(forward, "case_manifest_hash", "forward requests") == manifest_hash
requests = forward["requests"]
mismatch("forward requests must be a sequence") unless requests.is_a?(Psych::Nodes::Sequence)
mismatch("forward requests count differs") unless requests.children.length == case_ids.length
raw_by_id = {}
requests.children.each_with_index do |request_node, index|
  request = mapping(request_node, "forward requests[#{index}]")
  exact_keys(request, %w[id raw_request], "forward requests[#{index}]")
  id = scalar(request, "id", "forward requests[#{index}]")
  raw = scalar(request, "raw_request", "forward requests[#{index}]")
  mismatch("forward request order differs") unless id == case_ids[index]
  raw_by_id[id] = raw
end

stages = mapping(state["stages"], "stages")
stage_50_entries = mapping(stages["50"], "stages.50")
stage_50_status = scalar(stage_50_entries, "status", "stages.50")
allowed_stage_50_statuses = %w[pending complete blocked not_applicable]
mismatch("invalid stages.50.status") unless allowed_stage_50_statuses.include?(stage_50_status)

if stage_50_status == "complete"
  forward_test = mapping(evaluation["forward_test"], "evaluation.forward_test")
  exact_keys(forward_test, %w[cases controller_session_id], "evaluation.forward_test")
  controller_session_id = scalar(forward_test, "controller_session_id", "evaluation.forward_test")
  outcomes = forward_test["cases"]
  mismatch("evaluation.forward_test.cases must be a sequence") unless outcomes.is_a?(Psych::Nodes::Sequence)
  mismatch("evaluation.forward_test.cases count differs") unless outcomes.children.length == case_ids.length
  child_session_ids = []
  outcomes.children.each_with_index do |outcome_node, index|
    outcome = mapping(outcome_node, "evaluation.forward_test.cases[#{index}]")
    exact_keys(outcome, %w[child_session_id id outcome request_hash], "evaluation.forward_test.cases[#{index}]")
    id = scalar(outcome, "id", "evaluation.forward_test.cases[#{index}]")
    mismatch("evaluation.forward_test case order differs") unless id == case_ids[index]
    child_session_id = scalar(outcome, "child_session_id", "evaluation.forward_test.cases[#{index}]")
    mismatch("evaluation.forward_test child identity equals controller") if child_session_id == controller_session_id
    mismatch("evaluation.forward_test child identity is reused") if child_session_ids.include?(child_session_id)
    child_session_ids << child_session_id
    request_hash = scalar(outcome, "request_hash", "evaluation.forward_test.cases[#{index}]")
    mismatch("evaluation.forward_test request hash differs") unless request_hash == Digest::SHA256.hexdigest(raw_by_id.fetch(id))
    outcome_value = scalar(outcome, "outcome", "evaluation.forward_test.cases[#{index}]")
    mismatch("invalid evaluation.forward_test outcome") unless %w[CONFIRMED PARTIAL REJECTED INCONCLUSIVE].include?(outcome_value)
  end
end

puts "PASS: evaluation contract dva-routing-v1 #{manifest_hash} #{forward_hash}"
RUBY
}

check_fixture() {
  local fixture=$1
  ruby -rpsych - "$fixture" <<'RUBY'
path = ARGV.fetch(0)
def fail!(message)
  warn "FAIL: #{message}"
  exit 1
end

fixture = Psych.safe_load(File.read(path), aliases: false)
fail!("fixture must be a mapping") unless fixture.is_a?(Hash)
required = %w[authority boundary completion gate handoff lifecycle_commands mode next_prompt operations revisions run_dir sources stage]
fail!("fixture keys differ") unless fixture.keys.sort == required
%w[mode stage gate boundary run_dir next_prompt].each do |key|
  fail!("#{key} must be a non-empty string") unless fixture[key].is_a?(String) && !fixture[key].empty?
end
fail!("mode must be step or continuous") unless %w[step continuous].include?(fixture["mode"])
fail!("stage must be numbered or post_cycle_qa") unless fixture["stage"].match?(/\A(?:00|10|20|30|40|45|50|60|70|post_cycle_qa)\z/)
fail!("gate must be PASS, SKIPPED, or BLOCKED") unless %w[PASS SKIPPED BLOCKED].include?(fixture["gate"])
fail!("boundary invalid") unless %w[accepted blocked fresh_session authority completion].include?(fixture["boundary"])
fail!("run_dir must be absolute") unless fixture["run_dir"].start_with?("/")
fail!("next_prompt must be a prompt token") unless fixture["next_prompt"].match?(/\A(?:[0-9]{2}-[a-z-]+\.md|post-cycle-qa\.md|none)\z/)
handoff = fixture["handoff"]
fail!("handoff must reproduce RUN_DIR and NEXT_PROMPT") unless handoff.is_a?(Hash) && handoff.keys.sort == %w[next_prompt run_dir] && handoff["run_dir"] == fixture["run_dir"] && handoff["next_prompt"] == fixture["next_prompt"]
fail!("accepted boundary requires PASS or SKIPPED") if fixture["boundary"] == "accepted" && !%w[PASS SKIPPED].include?(fixture["gate"])
fail!("blocked/fresh-session/authority boundary requires BLOCKED") if %w[blocked fresh_session authority].include?(fixture["boundary"]) && fixture["gate"] != "BLOCKED"
fail!("completion boundary must name post-cycle QA or none") if fixture["boundary"] == "completion" && !%w[post-cycle-qa.md none].include?(fixture["next_prompt"])

transitions = {
  ["00", "accepted"] => %w[10-audit-skill.md],
  ["10", "accepted"] => %w[20-capture-baseline.md],
  ["20", "accepted"] => %w[30-improve-skill.md 40-improve-prompts.md 45-improve-dva-tool.md 50-apply-to-project.md 60-evaluate.md],
  ["30", "accepted"] => %w[40-improve-prompts.md 50-apply-to-project.md],
  ["30", "fresh_session"] => %w[40-improve-prompts.md],
  ["40", "accepted"] => %w[50-apply-to-project.md],
  ["45", "accepted"] => %w[50-apply-to-project.md],
  ["50", "accepted"] => %w[60-evaluate.md],
  ["50", "authority"] => %w[post-cycle-qa.md],
  ["60", "accepted"] => %w[30-improve-skill.md 40-improve-prompts.md 45-improve-dva-tool.md 70-feedback.md],
  ["70", "completion"] => %w[post-cycle-qa.md none],
  ["post_cycle_qa", "accepted"] => %w[none]
}
expected_next = transitions.fetch([fixture["stage"], fixture["boundary"]], fixture["boundary"] == "blocked" ? %w[00-start-cycle.md] : nil)
fail!("invalid stage/boundary transition") if expected_next.nil? || !expected_next.include?(fixture["next_prompt"])

sources = fixture["sources"]
fail!("sources keys differ") unless sources.is_a?(Hash) && sources.keys.sort == %w[config_projection packages_root]
fail!("packages_root must be an absolute literal") unless sources["packages_root"].is_a?(String) && sources["packages_root"].start_with?("/")
fail!("config projection must be active or missing") unless %w[active missing].include?(sources["config_projection"])
revisions = fixture["revisions"]
fail!("revisions keys differ") unless revisions.is_a?(Hash) && revisions.keys.sort == %w[packages_dirty_hash packages_head packages_protected_dirty_paths]
fail!("packages_head must be a Git SHA") unless revisions["packages_head"].is_a?(String) && revisions["packages_head"].match?(/\A[0-9a-f]{40}\z/)
fail!("packages_dirty_hash must be SHA-256") unless revisions["packages_dirty_hash"].is_a?(String) && revisions["packages_dirty_hash"].match?(/\A[0-9a-f]{64}\z/)
fail!("protected dirty paths must be non-empty path names") unless revisions["packages_protected_dirty_paths"].is_a?(Array) && !revisions["packages_protected_dirty_paths"].empty? && revisions["packages_protected_dirty_paths"].all? { |value| value.is_a?(String) && !value.empty? }
operations = fixture["operations"]
fail!("operations must declare config invocation") unless operations.is_a?(Hash) && operations.keys == ["config_invocation_required"] && [true, false].include?(operations["config_invocation_required"])
if sources["config_projection"] == "missing" && operations["config_invocation_required"]
  fail!("missing config projection must block the requiring operation") unless fixture["gate"] == "BLOCKED" && fixture["boundary"] == "blocked"
end

authority = fixture["authority"]
fail!("authority keys differ") unless authority.is_a?(Hash) && authority.keys.sort == %w[approved commands scope]
fail!("authority scope invalid") unless %w[numbered_stage post_cycle_qa].include?(authority["scope"])
fail!("authority approved must be boolean") unless [true, false].include?(authority["approved"])
fail!("authority.commands must be a sequence") unless authority["commands"].is_a?(Array)
allowed_effects = {
  "dva provision" => "provisions_public_directories",
  "dva up full-stack" => "starts_full_stack_services",
  "dva down full-stack" => "stops_cycle_started_full_stack_services"
}
authority["commands"].each do |entry|
  fail!("authority command entry must be structured") unless entry.is_a?(Hash) && entry.keys.sort == %w[command side_effect]
  fail!("authority command entry values must be non-empty") unless entry.values.all? { |value| value.is_a?(String) && !value.empty? }
  fail!("authority command has undeclared side effect") unless allowed_effects[entry["command"]] == entry["side_effect"]
end
if authority["approved"]
  fail!("generic runtime approval is insufficient") unless authority["scope"] == "post_cycle_qa" && !authority["commands"].empty?
else
  fail!("unapproved authority cannot list commands") unless authority["commands"].empty?
end

lifecycle = fixture["lifecycle_commands"]
fail!("lifecycle_commands must be scalar sequence") unless lifecycle.is_a?(Array) && lifecycle.all? { |value| value.is_a?(String) && !value.empty? }
fail!("numbered stage contains lifecycle command") if fixture["stage"] != "post_cycle_qa" && !lifecycle.empty?
authorized_commands = authority["commands"].map { |entry| entry["command"] }
fail!("post-cycle lifecycle requires exact approved authority") if fixture["stage"] == "post_cycle_qa" && !lifecycle.empty? && !(authority["approved"] && authority["scope"] == "post_cycle_qa" && authorized_commands.sort == lifecycle.sort)
fail!("completion must be boolean") unless [true, false].include?(fixture["completion"])
fail!("completion boundary requires completion=true") if fixture["boundary"] == "completion" && !fixture["completion"]
puts "RUN_DIR=#{fixture["run_dir"]}"
puts "NEXT_PROMPT=#{fixture["next_prompt"]}"
puts "PASS: fixture #{File.basename(path)} #{fixture["stage"]}/#{fixture["boundary"]}"
RUBY
}

check_docs() {
  local artifacts="$ROOT/ref-artifacts.md"

  grep -Fq 'primary_owner: null # plugin | local_setup | target | environment | no_change' "$artifacts" ||
    fail "DVA state redefines or omits the generic run.primary_owner enum"
  grep -Fq 'owner_route: null # skill | prompt | dva_tool | target_project | environment | no_change' "$artifacts" ||
    fail "missing DVA-only run.owner_route enum"
  grep -Fq 'mode: continuous # continuous | step' "$artifacts" || fail "missing run.mode"
  grep -Fq 'predecessor_run_id: null' "$artifacts" || fail "missing run.predecessor_run_id"
  grep -Fq '<!-- contract:safety forward_test_target=read_only target_owner_application=reversible_patch pre_edit_patch=required inverse=required lifecycle=forbidden irreversible=forbidden -->' "$ROOT/METHODOLOGY.md" ||
    fail "missing structural safety contract"
  grep -Fq '<!-- contract:step accepted=PASS|SKIPPED stop=required emit=RUN_DIR,NEXT_PROMPT blocked=stop fresh_session=stop authority=stop completion=stop -->' "$ROOT/METHODOLOGY.md" ||
    fail "missing structural step boundary contract"
  grep -Fq '<!-- contract:numbered-stages ids=00,10,20,30,40,45,50,60,70 lifecycle=forbidden real_target_lifecycle=forbidden post_cycle_qa=separate -->' "$ROOT/METHODOLOGY.md" ||
    fail "missing structural numbered lifecycle contract"
  grep -Fq '<!-- contract:runtime-authority generic=insufficient exact_command=required side_effect=required scope=post_cycle_qa -->' "$ROOT/METHODOLOGY.md" ||
    fail "missing structural runtime authority contract"
  grep -Fq '<!-- contract:owner-selection primary=generic route=dva secondary=successor -->' "$ROOT/20-capture-baseline.md" ||
    fail "missing structural baseline owner contract"
  grep -Fq '<!-- contract:owner-evaluation same_primary=reenter different_primary=successor predecessor=required -->' "$ROOT/60-evaluate.md" ||
    fail "missing structural evaluation owner contract"
  grep -Fq '<!-- contract:owner-feedback different_primary=successor predecessor=required post_evaluation_mutation=forbidden -->' "$ROOT/70-feedback.md" ||
    fail "missing structural feedback owner contract"
  grep -Fq '<!-- contract:evaluation-freeze version=dva-routing-v1 manifest=ordered hash=sha256 requests=one_raw_per_case leakage=forbidden mismatch=successor -->' "$ROOT/20-capture-baseline.md" ||
    fail "missing structural evaluation freeze contract"
  grep -Fq '<!-- contract:forward-test controller=required child=history_free fixture=disposable target=read_only lifecycle=forbidden results=one_per_case -->' "$ROOT/50-apply-to-project.md" ||
    fail "missing structural forward-test contract"
  grep -Fq '<!-- contract:forward-test-identities child=unique child_ne_controller=true stage50_status=pending|complete|blocked|not_applicable complete=all_results -->' "$ROOT/50-apply-to-project.md" ||
    fail "missing structural forward-test identity/status contract"
  grep -Eq '<!-- evaluation-manifest:sha256=[0-9a-f]{64} -->' "$ROOT/ref-evaluation.md" ||
    fail "missing deterministic evaluation manifest SHA-256"

  local stage
  for stage in 00 10 20 30 40 45 50 60 70; do
    grep -Eq "<!-- contract:stage id=${stage} mode_step=stop emit=RUN_DIR,NEXT_PROMPT numbered_lifecycle=forbidden real_target_lifecycle=forbidden -->" "$ROOT/${stage}-"*.md ||
      fail "missing structural stage boundary contract for ${stage}"
  done
  rg -n --glob '*.md' --glob '!fixtures/**' '/Users/archmagece/devenv' "$ROOT" >/dev/null &&
    fail "workflow contains a reusable hardcoded packages root"

  ruby -rpsych - "$artifacts" <<'RUBY'
source = File.read(ARGV.fetch(0))
yaml = source.match(/```yaml\n(.*?)\n```/m)&.captures&.first
abort "FAIL: missing state schema YAML block" if yaml.nil?
root = Psych.safe_load(yaml, aliases: false)
abort "FAIL: state schema root must be a mapping" unless root.is_a?(Hash)
run = root["run"]
evaluation = root["evaluation"]
abort "FAIL: state schema run must be a mapping" unless run.is_a?(Hash)
abort "FAIL: state schema evaluation must be a mapping" unless evaluation.is_a?(Hash)
abort "FAIL: incomplete run state fields" unless %w[mode primary_owner owner_route predecessor_run_id].all? { |key| run.key?(key) }
abort "FAIL: incomplete evaluation state fields" unless %w[version case_ids case_manifest_hash forward_requests_hash].all? { |key| evaluation.key?(key) }
forward_test = evaluation["forward_test"]
abort "FAIL: incomplete forward-test state fields" unless forward_test.is_a?(Hash) && %w[controller_session_id cases].all? { |key| forward_test.key?(key) }
revisions = root["revisions"]
sources = root["sources"]
operations = root["operations"]
authority = root["authority"]
abort "FAIL: incomplete package provenance" unless revisions.is_a?(Hash) && %w[packages_head packages_dirty_hash packages_protected_dirty_paths].all? { |key| revisions.key?(key) }
abort "FAIL: packages root field missing" unless sources.is_a?(Hash) && sources.key?("packages_root")
abort "FAIL: incomplete config projection state" unless sources.key?("config_projection")
abort "FAIL: incomplete invocation state" unless operations.is_a?(Hash) && [true, false].include?(operations["config_invocation_required"])
abort "FAIL: incomplete authority state" unless authority.is_a?(Hash) && %w[scope approved commands].all? { |key| authority.key?(key) }
RUBY

  ruby -rdigest - "$ROOT/ref-evaluation.md" <<'RUBY'
source = File.binread(ARGV.fetch(0))
match = source.match(%r{<!-- evaluation-manifest:start -->\n```yaml\n(.*?)```\n<!-- evaluation-manifest:end -->}m)
abort "FAIL: missing canonical evaluation manifest" if match.nil?
declared = source.match(/<!-- evaluation-manifest:sha256=([0-9a-f]{64}) -->/)&.captures&.first
actual = Digest::SHA256.hexdigest(match[1])
abort "FAIL: evaluation manifest SHA-256 differs" unless declared == actual
RUBY

  printf 'PASS: DVA owner/state documentation contract\n'
}

if [[ "${1:-}" == "--state" ]]; then
  [[ $# == 2 ]] || fail "usage: $0 --state <state.yaml>"
  check_state "$2"
elif [[ "${1:-}" == "--evaluation" ]]; then
  [[ $# == 3 ]] || fail "usage: $0 --evaluation <state.yaml> <run-dir>"
  check_evaluation "$2" "$3"
elif [[ "${1:-}" == "--fixture" ]]; then
  [[ $# == 2 ]] || fail "usage: $0 --fixture <fixture.yaml>"
  check_fixture "$2"
else
  [[ $# == 0 ]] || fail "usage: $0 [--state <state.yaml> | --evaluation <state.yaml> <run-dir> | --fixture <fixture.yaml>]"
  check_docs
fi
