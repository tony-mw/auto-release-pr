# Go Auto Release Pull Request

## Description
- This is a CLI based tool that will create PR to a config repository to trigger a deployment to a staging environment

## Usage
- This is a standalone CLI, but was specifically developed to run in a Jenkins stage such as

``` groovy
steps {
    container(name: 'auto-release-pr') {
      withCredentials([
        usernamePassword(
        credentialsId: 'bitbucket-write',
        usernameVariable: 'USERNAME',
        passwordVariable: 'PASSWORD')
        ]) {
          sh "apk add git && apk add bash"
          sh "./infrastructure/scripts/format-git-credentials.sh > ~/.git-credentials"
          sh "./infrastructure/scripts/git-config.sh"
          script {
            servicesChanged = []
            println dirs
            for (i in dirs) {
              a = i.split("/")[1]
              servicesChanged.add(a)
            }
            myServicesString = servicesChanged.join(",")
          }
          sh "echo $USERNAME && auto-release-pr staging --bitbucket-project=scm --repo-slug=dpns-gitops-nonprod --source-branch=release/pcoe --services=${myServicesString} --product=products/outcome-simulation"
      }
    }
  }
```