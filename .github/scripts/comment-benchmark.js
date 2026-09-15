const fs = require('fs');

const marker = '<!-- performance-benchmark -->';

module.exports = async ({ github, context }) => {
  const results = fs.readFileSync('backend/benchmark-results.txt', 'utf8');
  const body = [
    marker,
    '## Performance Benchmark Results',
    '',
    `Commit ${context.payload.pull_request.head.sha}`,
    '',
    '```',
    results,
    '```',
    '',
    '*Performance regression tests must pass for PR to be merged.*',
  ].join('\n');

  const comments = await github.paginate(github.rest.issues.listComments, {
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
  });
  const existing = comments.find(
    (c) => c.user?.login === 'github-actions[bot]' && c.body?.includes(marker)
  );
  if (existing) {
    await github.rest.issues.updateComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      comment_id: existing.id,
      body,
    });
    return;
  }

  await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
    body,
  });
};
