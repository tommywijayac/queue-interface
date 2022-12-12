const warning = document.getElementById("warning");

warning.addEventListener("animationend", function() {
    warning.style.display = "none";
});

function ShowWarning(input) {
    warning.innerHTML = "antrian harus diawali dengan huruf, diikuti dengan tiga angka (contoh: A001)";
    warning.style.display = "block";

    warning.classList.remove("fadein-out");
    // Trigger reflow. 'Magic' in order for animation can be triggered on every click
    warning.offsetWidth = warning.offsetWidth;
    warning.classList.add("fadein-out");
}

function QueueNumberInput() {
    const inputs = document.querySelectorAll('#qnum > *[id]');
    for (let i = 0; i < inputs.length; i++) {
        // Requirement: First character must be character, the other must be number
        if (i === 0) {
            inputs[i].addEventListener('keypress', function(event) {
                if ((event.keyCode > 64 && event.keyCode < 91) || (event.keyCode > 96 && event.keyCode < 123)) {
                    inputs[i].value = String.fromCharCode(event.keyCode);
                    if (i !== inputs.length - 1)
                        inputs[i + 1].focus();
                    event.preventDefault();
                } else {
                    ShowWarning(inputs[i]);
                    event.preventDefault();
                }
            });
        } else {
            inputs[i].addEventListener('keydown', function(event) {
                if (event.key == "Backspace") {
                    if (i == inputs.length - 1 && inputs[i].value !== '') {
                        // don't go back to previous block
                        // must be emptied inside if because value is used for evaluation
                        inputs[i].value = '';
                    } else if (i != 0) {
                        inputs[i].value = '';
                        inputs[i-1].focus();
                    }
                } else if (event.key == "Enter") {
                    // Pass-through, else got processed and considered wrong combination
                    return true;
                } else {
                    if (event.keyCode > 47 && event.keyCode < 58) {
                        inputs[i].value = event.key;
                        if (i !== inputs.length - 1)
                            inputs[i + 1].focus();
                        event.preventDefault();
                    } else {
                        ShowWarning(inputs[i]);
                        event.preventDefault();
                    }
                }
            });
        }
    }
}
QueueNumberInput();

function updateProcess() {
    var e = document.getElementById("branch");
    var selectedBranch = e.value;

    switch (selectedBranch){
        case '':
        case 'kbj':
        case 'jsl':
        case 'kmy':
        case 'smg':
            document.getElementById("opr").classList.remove("disabled");
            break;
        default:
            document.getElementById("opr").classList.add("disabled");
    }
}

function ShowBranchWarning() {
  warning.innerHTML = "mohon pilih lokasi terlebih dahulu";
  warning.style.display = "block";

  warning.classList.remove("fadein-out");
  // Trigger reflow. 'Magic' in order for animation can be triggered on every click
  warning.offsetWidth = warning.offsetWidth;
  warning.classList.add("fadein-out");
}

function Search(e) {
  var branch = document.getElementById("branch").value
  if (branch === '') {
    ShowBranchWarning();

    //abort process
    e.preventDefault();
    return false;
  }

  //continue with the default process
  return true;
}

var form = document.getElementById("search")
if (form.attachEvent) {
  form.attachEvent("submit", Search);
} else {
  form.addEventListener("submit", Search);
}