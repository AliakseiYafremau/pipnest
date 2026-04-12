//go:build linux || darwin
// +build linux darwin

package cheatsheet

import "strings"

// CheatCommand describes one command snippet shown in the cheatsheet.
type CheatCommand struct {
	Category    string
	Command     string
	Description string
}

// CheatCommands is the curated command list shown in the cheatsheet screen.
var CheatCommands = []CheatCommand{
	// Pip - Package Management
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip install <package>",
		Description: "Install a Python package",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip install -U <package>",
		Description: "Upgrade an installed package",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip uninstall <package>",
		Description: "Remove a package",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip list",
		Description: "Show all installed packages",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip freeze > requirements.txt",
		Description: "Export dependencies to requirements.txt",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip install -r requirements.txt",
		Description: "Install from requirements file",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip show <package>",
		Description: "Show package details",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip cache purge",
		Description: "Clear pip cache",
	},
	{
		Category:    "📦 Pip - Package Manager",
		Command:     "pip check",
		Description: "Verify all dependencies are compatible",
	},

	// Virtual Environments
	{
		Category:    "🐍 Virtual Environments",
		Command:     "python -m venv <venv_name>",
		Description: "Create a new virtual environment",
	},
	{
		Category:    "🐍 Virtual Environments",
		Command:     "source <venv_name>/bin/activate",
		Description: "Activate venv on Linux/Mac",
	},
	{
		Category:    "🐍 Virtual Environments",
		Command:     "<venv_name>\\Scripts\\activate",
		Description: "Activate venv on Windows",
	},
	{
		Category:    "🐍 Virtual Environments",
		Command:     "deactivate",
		Description: "Deactivate current venv",
	},
	{
		Category:    "🐍 Virtual Environments",
		Command:     "rm -rf <venv_name>",
		Description: "Delete virtual environment",
	},
	{
		Category:    "🐍 Virtual Environments",
		Command:     "which python",
		Description: "Show active Python interpreter path",
	},

	// Python
	{
		Category:    "🔧 Python",
		Command:     "python --version",
		Description: "Show Python version",
	},
	{
		Category:    "🔧 Python",
		Command:     "python -m pip --version",
		Description: "Show pip version",
	},
	{
		Category:    "🔧 Python",
		Command:     "python -c \"import <module>\"",
		Description: "Test if module is installed",
	},
	{
		Category:    "🔧 Python",
		Command:     "python -m <module>",
		Description: "Run module as script",
	},
	{
		Category:    "🔧 Python",
		Command:     "python -i <script>.py",
		Description: "Run script and enter interactive mode",
	},
	{
		Category:    "🔧 Python",
		Command:     "python -m pdb <script>.py",
		Description: "Run script with debugger",
	},

	// Requirements Management
	{
		Category:    "📋 Requirements",
		Command:     "pip install pipreqs",
		Description: "Install tool to auto-generate requirements",
	},
	{
		Category:    "📋 Requirements",
		Command:     "pipreqs . --force",
		Description: "Generate requirements from imports",
	},
	{
		Category:    "📋 Requirements",
		Command:     "pip list --outdated",
		Description: "Check for outdated packages",
	},
	{
		Category:    "📋 Requirements",
		Command:     "pip install pip-tools",
		Description: "Install pip-tools for better dependency management",
	},
	{
		Category:    "📋 Requirements",
		Command:     "pip-compile requirements.in",
		Description: "Compile requirements.in to requirements.txt",
	},

	// Development Tools
	{
		Category:    "🛠️ Development Tools",
		Command:     "pip install black",
		Description: "Code formatter",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "black <file>.py",
		Description: "Format code with Black",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "pip install flake8",
		Description: "Code linter",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "flake8 <file>.py",
		Description: "Lint code with Flake8",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "pip install pytest",
		Description: "Testing framework",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "pytest <file>.py",
		Description: "Run tests with Pytest",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "pip install mypy",
		Description: "Static type checker",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "mypy <file>.py",
		Description: "Type check with MyPy",
	},
	{
		Category:    "🛠️ Development Tools",
		Command:     "pip install ipython",
		Description: "Enhanced Python shell",
	},

	// Utilities
	{
		Category:    "⚙️ Utilities",
		Command:     "pip install virtualenv",
		Description: "Alternative virtual environment tool",
	},
	{
		Category:    "⚙️ Utilities",
		Command:     "pip install poetry",
		Description: "Modern dependency management",
	},
	{
		Category:    "⚙️ Utilities",
		Command:     "poetry install",
		Description: "Install dependencies with Poetry",
	},
	{
		Category:    "⚙️ Utilities",
		Command:     "pip install wheel twine",
		Description: "Tools for building packages",
	},
	{
		Category:    "⚙️ Utilities",
		Command:     "python -m build",
		Description: "Build Python package",
	},
	{
		Category:    "⚙️ Utilities",
		Command:     "twine upload dist/*",
		Description: "Upload package to PyPI",
	},
}

// FilterCommands filters commands by command text, description, or category.
func FilterCommands(commands []CheatCommand, search string) []CheatCommand {
	if search == "" {
		return commands
	}

	search = strings.ToLower(search)
	var filtered []CheatCommand

	for _, cmd := range commands {
		if strings.Contains(strings.ToLower(cmd.Command), search) ||
			strings.Contains(strings.ToLower(cmd.Description), search) ||
			strings.Contains(strings.ToLower(cmd.Category), search) {
			filtered = append(filtered, cmd)
		}
	}

	return filtered
}
