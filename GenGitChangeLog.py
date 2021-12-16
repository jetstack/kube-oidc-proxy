# Generates OpenUnison Changelog
# Call from the branch with 3 parameters:
# 1. Date from which to start looking
# 2. Github Token

# requires python-dateutil and requests from pip

from subprocess import *
import re
from datetime import datetime
import dateutil.parser
import sys
import requests



def parseIssues(message):
    issuesRet = []
    issues = re.findall('[#][0-9]+',message)
    if issues != None:
        for issue in issues:
            issuesRet.append(issue[1:])
    return issuesRet


def f4(seq):
   # order preserving
   noDupes = []
   [noDupes.append(i) for i in seq if not noDupes.count(i)]
   return noDupes






headers = {'Authorization':'token ' + sys.argv[2]}


GIT_COMMIT_FIELDS = ['id', 'author_name', 'author_email', 'date', 'message']
GIT_LOG_FORMAT = ['%H', '%an', '%ae', '%ai', '%s']
GIT_LOG_FORMAT = '%x1f'.join(GIT_LOG_FORMAT) + '%x1e'

#print repo.git.log(p=False)

allIssues = []

p = Popen('git log --format="%s" ' % GIT_LOG_FORMAT, shell=True, stdout=PIPE)
(logb, _) = p.communicate()
log = str(logb,"utf-8")
log = log.strip('\n\x1e').split("\x1e")
log = [row.strip().split("\x1f") for row in log]
log = [dict(zip(GIT_COMMIT_FIELDS, row)) for row in log]

notbefore = dateutil.parser.parse(sys.argv[1] + ' 00:00:00 -0400')

for commit in log:
    created = dateutil.parser.parse(commit['date'])
    if created > notbefore:
        message = commit['message']
        allIssues.extend(parseIssues(message))


allIssues = f4(allIssues)

bylabels = {}

for issue in allIssues:
    issueURL = 'https://api.github.com/repos/TremoloSecurity/kube-oidc-proxy/issues/' + issue
    r = requests.get(issueURL,headers=headers)
    json = r.json();
    
    if "labels" in json:
        for label in json['labels']:
            if not (label['name'] in bylabels):
                labelGroup = []
                bylabels[label["name"]] = labelGroup
            labelGroup = bylabels[label['name']]
            labelGroup.append(json)


for label in bylabels:
    print('**' + label + 's:**')
    for issue in bylabels[label]:
        print(' - ' + issue['title'] + ' [\\#' + str(issue['number']) + '](' + issue['html_url'] + ')')
    print()
