#!/bin/bash
set +e  # Continue on errors

COLOR_BLUE="\033[0;94m"
COLOR_GREEN="\033[0;92m"
COLOR_RESET="\033[0m"

# Print useful output for user
#!/bin/bash
set +e  # Continue on errors

COLOR_BLUE="\033[0;94m"
COLOR_GREEN="\033[0;92m"
COLOR_RESET="\033[0m"

cat <<EOF
${COLOR_BLUE}
     %########%
     %#########%  ██╗  ██╗ ██████╗ ██████╗ ███████╗██╗
     %#########%  ██║  ██ ██╔═══██╗██╔══██╗██╔════╝██║
     %#########%  ███████║██║   ██║██║  ██║█████╗  ██║
     %#########%  ██╔══██║██║   ██║█ ║  ██║██╔══╝  ██║
     %#########%  ██║  ██║╚██████╔╝██████╔╝███████╗██║
     %#########%  ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚═╝

     %#########%  ██████╗ ███████╗██╗   ██╗ ██████╗ █████╗ ███████╗
     %#########%  ██╔══██╗██╔════╝██║   ██║██╔═══██╗██╔══██╗██╔════╝
     %#########%  ██║  ██║█████╗  ██║   ██║██║   ██║██████╔╝███████╗
     %#########%  ██║  ██║██╔══╝  ╚██╗ ██╔╝██║   ██║██╔═══╝ ╚════██║
     %#########%  ██████╔╝███████╗ ╚███╔╝ ╚██████╔╝██║     ███████║
     %#########%  ╚═════╝ ╚══════╝  ╚═══╝   ╚═════╝ ╚═╝     ╚══════╝
 %###############%                                  |_|
 %###########%${COLOR_RESET}

Welcome to your development container!

This is how you can work with it:
- Files will be synchronized between your local machine and this container
- Some ports will be forwarded, so you can access this container via localhost
- Run ${COLOR_GREEN}go run main.go${COLOR_RESET} to start the application
EOF

# Set terminal prompt
export PS1="\[${COLOR_BLUE}\]devspace\[${COLOR_RESET}\] ./\W \[${COLOR_BLUE}\]\\$\[${COLOR_RESET}\] "
if [ -z "$BASH" ]; then export PS1="$ "; fi

# Include project's bin/ folder in PATH
export PATH="./bin:$PATH"

# Open shell
bash --norc
# Set terminal prompt
export PS1="\[${COLOR_BLUE}\]devspace\[${COLOR_RESET}\] ./\W \[${COLOR_BLUE}\]\\$\[${COLOR_RESET}\] "
if [ -z "$BASH" ]; then export PS1="$ "; fi

# Include project's bin/ folder in PATH
export PATH="./bin:$PATH"

# Open shell
bash --norc
