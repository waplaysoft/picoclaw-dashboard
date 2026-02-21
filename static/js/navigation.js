// Navigation handling
document.addEventListener('DOMContentLoaded', () => {
    const navLinks = document.querySelectorAll('.nav-links a');

    navLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            // Убираем active у всех ссылок
            navLinks.forEach(l => l.classList.remove('active'));
            // Добавляем active на текущую
            link.classList.add('active');
        });
    });

    // Подсветка активной страницы при загрузке
    const currentPath = window.location.pathname;
    navLinks.forEach(link => {
        if (link.getAttribute('href') === currentPath) {
            link.classList.add('active');
        }
    });
});
