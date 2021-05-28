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
            inputs[i].addEventListener('keydown', function(event) {
                if (event.keyCode > 64 && event.keyCode < 91) {
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

document.getElementById("search-form").setAttribute("action", window.location.pathname);